package secrethub

import (
	"github.com/secrethub/secrethub-go/pkg/secrethub"
	"github.com/secrethub/secrethub-go/pkg/secrethub/configdir"
	"github.com/secrethub/secrethub-go/pkg/secrethub/credentials"
	"net/http"
	"net/url"
	"strings"
)

// Errors
var (
	ErrUnknownIdentityProvider = errMain.Code("unknown_identity_provider").ErrorPref("%s is not a supported identity provider. Valid options are `aws`, `gcp` and `key`.")
)

// ClientFactory handles creating a new client with the configured options.
type ClientFactory interface {
	// NewClient returns a new SecretHub client.
	NewClient() (secrethub.ClientInterface, error)
}

// NewClientFactory creates a new ClientFactory.
func NewClientFactory(store CredentialConfig) ClientFactory {
	return &clientFactory{
		identityProvider: "key",
		store:            store,
	}
}

type clientFactory struct {
	client           *secrethub.Client
	ServerURL        *url.URL
	identityProvider string
	proxyAddress     *url.URL
	store            CredentialConfig
}

// NewClient returns a new client that is configured to use the remote that
// is set with the flag.
func (f *clientFactory) NewClient() (secrethub.ClientInterface, error) {
	if f.client == nil {
		credentialProvider := f.store.Provider()
		switch strings.ToLower(f.identityProvider) {
		case "aws":
			credentialProvider = credentials.UseAWS()
		case "gcp":
			credentialProvider = credentials.UseGCPServiceAccount()
		case "key":
			credentialProvider = f.store.Provider()
		default:
			return nil, ErrUnknownIdentityProvider(f.identityProvider)
		}

		options := f.baseClientOptions()
		options = append(options, secrethub.WithCredentials(credentialProvider))

		client, err := secrethub.NewClient(options...)
		if err == configdir.ErrCredentialNotFound {
			return nil, ErrCredentialNotExist
		} else if err != nil {
			return nil, err
		}
		f.client = client
	}
	return f.client, nil
}

func (f *clientFactory) baseClientOptions() []secrethub.ClientOption {
	options := []secrethub.ClientOption{
		secrethub.WithConfigDir(f.store.ConfigDir()),
		secrethub.WithAppInfo(&secrethub.AppInfo{
			Name:    "secrethub-cli",
			Version: Version,
		}),
	}

	if f.proxyAddress != nil {
		transport := http.DefaultTransport.(*http.Transport)
		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			return f.proxyAddress, nil
		}
		options = append(options, secrethub.WithTransport(transport))
	}

	if f.ServerURL != nil {
		options = append(options, secrethub.WithServerURL(f.ServerURL.String()))
	}

	return options
}
