package proxy

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/inconshreveable/muxado"
	ezenv "github.com/jkassis/ezgo/env"
	ms "github.com/multiformats/go-multistream"
	log "github.com/sirupsen/logrus"
)

var proxyProxeeServiceAdvertisedHost string
var proxyProxeeServiceAdvertisedPort int64
var proxyAPIHTTPServiceBaseURL, proxyMyHostname string

const peerServiceProtocol = "/proxy/0.0.1"

var c http.Client

func init() {
	ezenv.ParseStr(&proxyMyHostname, "PROXY_HOSTNAME")

	ezenv.ParseStr(&proxyAPIHTTPServiceBaseURL, "PROXY_API_HTTP_SERVICE_BASEURL")

	ezenv.ParseStr(&proxyProxeeServiceAdvertisedHost, "PROXY_PROXEE_SERVICE_ADVERTISED_HOST")
	ezenv.ParseInt(&proxyProxeeServiceAdvertisedPort, "PROXY_PROXEE_SERVICE_ADVERTISED_PORT")
	rand.Seed(time.Now().UnixNano())
}

// Connect to the proxy
func Connect(hostname string, mux *ms.MultistreamMuxer) error {
	var streamMux muxado.Session
	streamMuxMut := sync.Mutex{}

	// ping forever
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			<-ticker.C

			err := func() error {
				streamMuxMut.Lock()
				streamMuxL := streamMux
				streamMuxMut.Unlock()

				if streamMuxL == nil {
					return nil
				}
				req, err := streamMuxL.Open()
				if err != nil {
					return fmt.Errorf("proxy.Client.Connect.ping: streamMux.Open: %v", err)
				}
				defer req.Close()

				// make a new multistream
				ms := ms.NewMSSelect(req, "/ping")
				defer ms.Close()

				// send the ping req
				_, err = fmt.Fprintln(ms, `ping`)
				if err != nil {
					return fmt.Errorf("proxy.Client.Connect.ping: send: %v", err)
				}

				// read the ping response
				scanner := bufio.NewScanner(ms)
				if !scanner.Scan() {
					return fmt.Errorf("proxy.Client.Connect.ping: could not read response... no input")
				}
				resp := scanner.Text()

				// validate it
				if resp != "pong" {
					return fmt.Errorf("proxy.Client.Connect.ping: expected 'pong' got '%s'", resp)
				}

				return nil
			}()

			if err != nil {
				log.Error(err)
			}
		}
	}()

	// dial forever with 10 second intervals
	for {
		log.Infof("Dialing %s", fmt.Sprintf("%s:%d", proxyProxeeServiceAdvertisedHost, proxyProxeeServiceAdvertisedPort))
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", proxyProxeeServiceAdvertisedHost, proxyProxeeServiceAdvertisedPort))
		if err != nil {
			log.Errorf("proxy.Client.Connect: %v", err)
			time.Sleep(10*time.Second + time.Duration(rand.Int63n(int64(2*time.Second))))
			continue
		}

		// register self and handle requests
		func() {
			streamMuxMut.Lock()
			streamMux = muxado.Client(conn, new(muxado.Config))
			streamMuxMut.Unlock()

			defer func() {
				streamMuxMut.Lock()
				streamMux.Close()
				streamMux = nil
				streamMuxMut.Unlock()
				conn.Close()
			}()

			// register ourselves
			req, err := streamMux.Open()
			if err != nil {
				log.Errorf("proxy.Client.Connect: streamMux.Open: %v", err)
				return
			}
			ms := ms.NewMSSelect(req, "/register")
			defer ms.Close()
			defer req.Close()

			// send the register req
			log.Infof("proxy.Client.Connect: register: sending hostname: %s", hostname)
			_, err = fmt.Fprintln(ms, hostname)
			if err != nil {
				log.Errorf("proxy.Client.Connect: %v", err)
				return
			}

			// read the register response
			scanner := bufio.NewScanner(ms)
			if !scanner.Scan() {
				log.Errorf("proxy.Client.Connect: register: no response")
				return
			}
			resp := scanner.Text()

			// validate it
			if resp != "OK" {
				log.Errorf("proxy.Client.Connect: register: expected 'OK' got '%s'", resp)
				return
			}
			log.Info("proxy.Client.Connect: register: success")

			// listen for requests and handle them
			for {
				stream, err := streamMux.Accept()
				if err != nil {
					log.Errorf("proxy.Client: mplex.Accept err: %v", err)
					return
				}

				go func() {
					err = mux.Handle(stream)
					if err != nil {
						log.Errorf("proxy.Client: mux.Handle: %v", err)
						return
					}
				}()
			}
		}()
	}
}
