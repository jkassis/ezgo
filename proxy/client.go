package proxy

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/inconshreveable/muxado"
	ezenv "github.com/jkassis/ezgo/env"
	"github.com/libp2p/go-libp2p-core/host"
	ms "github.com/multiformats/go-multistream"
	log "github.com/sirupsen/logrus"
)

var peerService host.Host

var proxyProxeeServiceAdvertisedHost string
var proxyProxeeServiceAdvertisedPort int64
var proxyAPIHTTPServiceBaseURL, proxyMyHostname string

const peerServiceProtocol = "/proxy/0.0.1"

var c http.Client

func init() {
	ezenv.ParseStr(&proxyMyHostname, "PROXY_HOSTNAME")

	ezenv.ParseStr(&proxyAPIHTTPServiceBaseURL, "PROXY_API_HTTP_SERVICE_BASEURL")

	ezenv.ParseStr(&proxyProxeeServiceAdvertisedHost, "PROXY_PROXEE_SERVICE_ADVERTISED_HOST")
	ezenv.ParseIntEnv(&proxyProxeeServiceAdvertisedPort, "PROXY_PROXEE_SERVICE_ADVERTISED_PORT")
	rand.Seed(time.Now().UnixNano())
}

// Connect to the proxy
func Connect(hostname string, mux *ms.MultistreamMuxer) error {
	// dial forever with 10 second intervals
	for {
		log.Infof("Dialing %s", fmt.Sprintf("%s:%d", proxyProxeeServiceAdvertisedHost, proxyProxeeServiceAdvertisedPort))
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", proxyProxeeServiceAdvertisedHost, proxyProxeeServiceAdvertisedPort))
		if err != nil {
			log.Errorf("proxy.Client.Connect: %s", err.Error())
			time.Sleep(10*time.Second + time.Duration(rand.Int63n(int64(2*time.Second))))
			continue
		}
		// mplex := multiplex.NewMultiplex(conn, true)
		mplex := muxado.Client(conn, new(muxado.Config))

		// Dial the proxy
		func() {
			defer mplex.Close()
			defer conn.Close()

			// register ourselves
			func() {
				req, err := mplex.Open()
				if err != nil {
					log.Errorf("proxy.Client.Connect: mplex.Open: %s", err.Error())
					return
				}
				rwc := ms.NewMSSelect(req, "/register")
				defer rwc.Close()
				defer req.Close()

				// send the req
				_, err = rwc.Write([]byte(hostname))
				if err != nil {
					log.Errorf("proxy.Client.Connect: %s", err.Error())
					return
				}
				rwc.Close()

				// read the response
				resp, err := ioutil.ReadAll(rwc)
				if err != nil {
					log.Errorf("proxy.Client.Connect: %s", err.Error())
					return
				}

				// validate response
				if !bytes.Equal(resp, []byte("OK")) {
					if err != nil {
						log.Errorf("proxy.Client.Connect: %s", err.Error())
						return
					}
				}
			}()

			for {
				stream, err := mplex.Accept()
				if err != nil {
					log.Errorf("proxy.Client: mplex.Accept err: %s", err.Error())
					return
				}

				go func() {
					err = mux.Handle(stream)
					if err != nil {
						log.Errorf("proxy.Client: mux.Handle: %s", err.Error())
						return
					}
				}()
			}
		}()

	}
}
