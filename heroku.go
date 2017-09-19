package heroku

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"os"
	"strings"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
)

// NewClusterConsumer creates a github.com/bsm/sarama-cluster.Consumer based on
// Heroku Kafka standard environment configs. Giving nil for cfg will create a
// generic config.
func NewClusterConsumer(groupID string, topics []string, cfg *cluster.Config) (*cluster.Consumer, error) {
	if cfg == nil {
		cfg = cluster.NewConfig()
	}

	// Configure TLS from environment
	tc, err := createTLSConfig()
	if err != nil {
		return nil, err
	}
	cfg.Net.TLS.Config = tc
	cfg.Net.TLS.Enable = true

	// Consumer groups require the Kafka prefix
	groupID = AppendPrefixTo(groupID)

	// Ensure all topics have the Kafka prefix applied
	for idx, topic := range topics {
		topics[idx] = AppendPrefixTo(topic)
	}

	brokers, err := Brokers()
	if err != nil {
		return nil, err
	}

	return cluster.NewConsumer(brokers, groupID, topics, cfg)
}

// NewConsumer creates a github.com/Shopify/sarama.Consumer configured from the
// standard Heroku Kafka environment.
func NewConsumer(cfg *sarama.Config) (sarama.Consumer, error) {
	if cfg == nil {
		cfg = sarama.NewConfig()
	}

	tc, err := createTLSConfig()
	if err != nil {
		return nil, err
	}
	cfg.Net.TLS.Config = tc
	cfg.Net.TLS.Enable = true

	brokers, err := Brokers()
	if err != nil {
		return nil, err
	}
	consumer, err := sarama.NewConsumer(brokers, cfg)
	if err != nil {
		return nil, err
	}

	return consumer, nil
}

// NewAsyncProducer creates a github.com/Shopify/sarama.AsyncProducer
// configured from the standard Heroku Kafka environment. When publishing
// messages to Multitenant Kafka all topics need to start with KAFKA_PREFIX
// which is best added using AppendPrefixTo.
func NewAsyncProducer(cfg *sarama.Config) (sarama.AsyncProducer, error) {
	if cfg == nil {
		cfg = sarama.NewConfig()
	}

	tc, err := createTLSConfig()
	if err != nil {
		return nil, err
	}
	cfg.Net.TLS.Config = tc
	cfg.Net.TLS.Enable = true

	brokers, err := Brokers()
	if err != nil {
		return nil, err
	}

	return sarama.NewAsyncProducer(brokers, cfg)
}

// NewSyncProducer creates a github.com/Shopify/sarama.SyncProducer configured
// from the standard Heroku Kafka environment. When publishing messages to
// Multitenant Kafka all topics need to start with KAFKA_PREFIX which is best
// added using AppendPrefixTo.
func NewSyncProducer(cfg *sarama.Config) (sarama.SyncProducer, error) {
	if cfg == nil {
		cfg = sarama.NewConfig()
	}

	tc, err := createTLSConfig()
	if err != nil {
		return nil, err
	}
	cfg.Net.TLS.Config = tc
	cfg.Net.TLS.Enable = true

	brokers, err := Brokers()
	if err != nil {
		return nil, err
	}

	return sarama.NewSyncProducer(brokers, cfg)
}

// AppendPrefixTo adds the env variable KAFKA_PREFIX to the given string if
// necessary. Heroku requires prefixing topics and consumer group ids with the
// prefix.
func AppendPrefixTo(str string) string {
	prefix := os.Getenv("KAFKA_PREFIX")

	if strings.HasPrefix(str, prefix) {
		return str
	}

	return prefix + str
}

// Create the TLS context, using the key and certificates provided.
func createTLSConfig() (*tls.Config, error) {
	trustedCert := os.Getenv("KAFKA_TRUSTED_CERT")
	if trustedCert == "" {
		return nil, errors.New("KAFKA_TRUSTED_CERT is not set in environment")
	}

	clientCertKey := os.Getenv("KAFKA_CLIENT_CERT_KEY")
	if clientCertKey == "" {
		return nil, errors.New("KAFKA_CLIENT_CERT_KEY is not set in environment")
	}

	clientCert := os.Getenv("KAFKA_CLIENT_CERT")
	if clientCert == "" {
		return nil, errors.New("KAFKA_CLIENT_CERT is not set in environment")
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(trustedCert))
	if !ok {
		return nil, errors.New("Invalid Root Cert. Please check your Heroku environment.")
	}

	// Setup certs for Sarama
	cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientCertKey))
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
		RootCAs:            roots,
	}, nil
}

// Brokers returns a list of host:port addresses for the Kafka brokers set in
// KAFKA_URL.
func Brokers() ([]string, error) {
	URL := os.Getenv("KAFKA_URL")
	if URL == "" {
		return nil, errors.New("KAFKA_URL is not set in environment")
	}

	urls := strings.Split(URL, ",")
	addrs := make([]string, len(urls))
	for i, v := range urls {
		u, err := url.Parse(v)
		if err != nil {
			return nil, err
		}

		// Validate the kafka+ssl url format. This simplifies our handling by
		// requiring a strict format that Heroku should provide for us.
		if u.Scheme != "kafka+ssl" {
			return nil, errors.New("kafka urls should start with kafka+ssl://")
		}

		addrs[i] = u.Host
	}

	return addrs, nil
}
