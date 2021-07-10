package proxee

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	mrand "math/rand"

	ezenv "github.com/jkassis/ezgo/env"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
)

var peerServiceDebug bool
var peerService host.Host

var peerServiceAdvertisedHost, peerServiceBindHost string
var peerServiceAdvertisedPort, peerServiceBindPort int64
var proxyAPIHTTPServiceBaseURL, proxyMyHostname string

const peerServiceProtocol = "/proxy/0.0.1"

var c http.Client

func init() {
	ezenv.ParseStr(&proxyMyHostname, "PROXY_HOSTNAME")
	ezenv.ParseStr(&proxyAPIHTTPServiceBaseURL, "PROXY_API_HTTP_SERVICE_BASEURL")
	ezenv.ParseStr(&peerServiceAdvertisedHost, "PEER_SERVICE_ADVERTISED_HOST")
	ezenv.ParseIntEnv(&peerServiceAdvertisedPort, "PEER_SERVICE_ADVERTISED_PORT")
	ezenv.ParseStr(&peerServiceBindHost, "PEER_SERVICE_BIND_HOST")
	ezenv.ParseIntEnv(&peerServiceBindPort, "PEER_SERVICE_BIND_PORT")
	ezenv.ParseBool(&peerServiceDebug, "PEER_SERVICE_DEBUG")
}

// Connect to the proxy
func Connect(streamHandler network.StreamHandler) error {
	var err error

	// If debug is enabled, use a constant random source to generate the peer ID. Only useful for debugging,
	// off by default. Otherwise, it uses rand.Reader.
	var randomness io.Reader
	if peerServiceDebug {
		// Use the port number as the randomness source.
		// This will always generate the same host ID on multiple executions, if the same port number is used.
		// Never do this in production code.
		randomness = mrand.New(mrand.NewSource(peerServiceBindPort))
	} else {
		randomness = rand.Reader
	}

	// Creates a new RSA key pair for this host.
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, randomness)
	if err != nil {
		return err
	}

	// bindHost 0.0.0.0 will listen on any interface device.
	peerServiceAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", peerServiceBindHost, peerServiceBindPort))

	// make my peer service
	peerService, err = libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(peerServiceAddr),
		libp2p.Identity(prvKey),
		// libp2p.NoSecurity,
	)

	if err != nil {
		return fmt.Errorf("proxyConnect: %w", err)
	}

	peerService.SetStreamHandler(peerServiceProtocol, streamHandler)

	log.Info("proxyConnect: peerService multiaddresses:")
	for _, la := range peerService.Addrs() {
		log.Infof("proxyConnect: - %v\n", la)
	}

	// register self with the proxy and get the proxy multi address
	// peerServiceMultiAddress := peerService.Addrs()[0].String()
	peerServiceMultiAddress := fmt.Sprintf("/ip4/%s/tcp/%v/p2p/%s", peerServiceAdvertisedHost, peerServiceAdvertisedPort, peerService.ID().Pretty())
	log.Infof("proxyConnect: peerService multi address is '%s'", peerServiceMultiAddress)

	URL, err := url.Parse(fmt.Sprintf("%s/proxeePut", proxyAPIHTTPServiceBaseURL))
	if err != nil {
		return fmt.Errorf("proxyConnect: failed to parse proxyApiHTTPServiceBaseURL: %w", err)
	}

	proxeePutReq := fmt.Sprintf(`{ "hostname": "%s", "p2pAddr": "%s" }`, proxyMyHostname, peerServiceMultiAddress)
	log.Infof("proxyConnect: connecting to proxy with %s:", proxeePutReq)
	req, err := http.NewRequest("POST", URL.String(), bytes.NewBuffer([]byte(proxeePutReq)))
	if err != nil {
		return fmt.Errorf("proxyConnect: %w", err)
	}
	res, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("proxyConnect: %w", err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("proxyConnect: unable to read from body: %v", err)
	}

	log.Infof("proxyConnect: proxy addr is '%s'", string(body))

	// host := makeRandomHost(*p2pport)

	// Turn the destination into a multiaddr.
	proxyPeerServiceMultiAddress, err := multiaddr.NewMultiaddr(string(body))
	if err != nil {
		return fmt.Errorf("proxyConnect: %w", err)
	}

	// get the peer ID for the proxy from the proxy multiaddress
	addrInfo, err := peer.AddrInfoFromP2pAddr(proxyPeerServiceMultiAddress)
	if err != nil {
		return fmt.Errorf("proxyConnect: %w", err)
	}

	// Add the proxy info the peerstore
	// This will be used during connection and stream creation by libp2p.
	log.Infof("proxyConnect: adding id: %s, and addr %s to the Peerstore", string(addrInfo.ID.Pretty()), addrInfo.Addrs[0].String())
	peerService.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, peerstore.PermanentAddrTTL)

	// Start a stream with the proxy. This will create the connection
	// that all streams between peerService and the proxyPeerService will share
	_, err = peerService.NewStream(context.Background(), addrInfo.ID, peerServiceProtocol)
	if err != nil {
		return fmt.Errorf("proxyConnect: %w", err)
	}
	log.Info("proxyConnect: Established peer connection to proxy")

	// nothing needs to be done with this stream.
	// the proxy will create new streams for each request

	// // Create a buffered stream so that read and writes are non blocking.
	// rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	return nil
}
