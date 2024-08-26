package service

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"

	"github.com/canonical/lxd/shared"
	"github.com/hashicorp/mdns"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudMDNS "github.com/canonical/microcloud/microcloud/mdns"
)

// Session represents a local trust establishment session.
type Session struct {
	lock       sync.RWMutex
	passphrase string
	server     *mdns.Server
	trustStore map[string]x509.Certificate

	joinIntentFingerprints []string
	joinIntents            chan types.SessionJoinPost
	exit                   chan bool
}

// generatePassphrase returns four random words chosen from wordlist.
// The words are separated by space.
func generatePassphrase() (string, error) {
	splitWordlist := strings.Split(wordlist, "\n")
	wordlistLength := int64(len(splitWordlist))

	var randomWords = make([]string, 4)
	for i := 0; i < 4; i++ {
		randomNumber, err := rand.Int(rand.Reader, big.NewInt(wordlistLength))
		if err != nil {
			return "", fmt.Errorf("Failed to get random number: %w", err)
		}

		splitLine := strings.Split(splitWordlist[randomNumber.Int64()], "\t")
		splitLineLength := len(splitLine)
		if splitLineLength != 2 {
			return "", fmt.Errorf("Invalid wordlist line: %q", splitWordlist[randomNumber.Int64()])
		}

		randomWords[i] = splitLine[1]
	}

	return strings.Join(randomWords, " "), nil
}

// NewSession returns a new local trust establishment session.
func NewSession(passphrase string) (*Session, error) {
	var err error

	if passphrase == "" {
		passphrase, err = generatePassphrase()
		if err != nil {
			return nil, err
		}
	}

	return &Session{
		passphrase: passphrase,
		trustStore: make(map[string]x509.Certificate),

		joinIntents: make(chan types.SessionJoinPost),
		exit:        make(chan bool),
	}, nil
}

// Passphrase returns the passphrase of the current trust establishment session.
func (s *Session) Passphrase() string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.passphrase
}

// Broadcast starts a new mDNS listener in the current trust establishment session.
func (s *Session) Broadcast(name string, address string, ifaceName string) error {
	info := cloudMDNS.ServerInfo{
		Version: cloudMDNS.Version,
		Name:    name,
		Address: address,
	}

	bytes, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("Failed to marshal server info: %w", err)
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("Failed to get interface %q by name: %w", ifaceName, err)
	}

	server, err := cloudMDNS.NewBroadcast(name, iface, address, int(CloudPort), cloudMDNS.ClusterService, bytes)
	if err != nil {
		return err
	}

	s.lock.Lock()
	s.server = server
	s.lock.Unlock()

	return nil
}

// Allow grants access via the temporary trust store to the given certificate.
func (s *Session) Allow(name string, cert x509.Certificate) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.trustStore[name] = cert
}

// TrustStore returns the temporary truststore of the current trust establishment session.
func (s *Session) TrustStore() map[string]x509.Certificate {
	s.lock.RLock()
	defer s.lock.RUnlock()

	// Create a copy of the trust store.
	trustStoreCopy := make(map[string]x509.Certificate)
	for name, cert := range s.trustStore {
		trustStoreCopy[name] = cert
	}

	return trustStoreCopy
}

// RegisterIntent registers the intention to join during the current trust establishment session
// for the given fingerprint.
func (s *Session) RegisterIntent(fingerprint string) error {
	if shared.ValueInSlice(fingerprint, s.joinIntentFingerprints) {
		return fmt.Errorf("Fingerprint already exists")
	}

	s.joinIntentFingerprints = append(s.joinIntentFingerprints, fingerprint)
	return nil
}

// IntentCh returns a channel which allows publishing and consuming join intents.
func (s *Session) IntentCh() chan types.SessionJoinPost {
	return s.joinIntents
}

// ExitCh returns a channel which allows waiting on the current trust establishment session.
func (s *Session) ExitCh() chan bool {
	return s.exit
}

// Stop stops the current trust establishment session.
func (s *Session) Stop() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.server != nil {
		err := s.server.Shutdown()
		if err != nil {
			return fmt.Errorf("Failed to shutdown mDNS server: %w", err)
		}
	}

	s.server = nil
	s.passphrase = ""
	s.trustStore = make(map[string]x509.Certificate, 0)
	s.joinIntentFingerprints = []string{}

	close(s.joinIntents)
	close(s.exit)

	return nil
}
