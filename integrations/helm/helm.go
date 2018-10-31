package helm

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8shelm "k8s.io/helm/pkg/helm"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/tlsutil"

	"github.com/weaveworks/flux/git"
)

const (
	GitOperationTimeout = 30 * time.Second
)

type RepoConfig struct {
	Repo       *git.Repo
	Branch     string
	ChartsPath string
}

type TillerOptions struct {
	Host      string
	Port      string
	Namespace string
	TLSVerify bool
	TLSEnable bool
	TLSKey    string
	TLSCert   string
	TLSCACert string
}

// Helm struct provides access to helm client
type Helm struct {
	logger log.Logger
	Host   string
	*k8shelm.Client
}

// NewClient creates a new helm client
func newClient(kubeClient *kubernetes.Clientset, opts TillerOptions) (*k8shelm.Client, error) {
	host, err := tillerHost(kubeClient, opts)
	if err != nil {
		return &k8shelm.Client{}, err
	}

	options := []k8shelm.Option{k8shelm.Host(host)}
	if opts.TLSVerify || opts.TLSEnable {
		tlscfg, err := tlsutil.ClientConfig(tlsutil.Options{
			KeyFile:            opts.TLSKey,
			CertFile:           opts.TLSCert,
			InsecureSkipVerify: !opts.TLSVerify,
			CaCertFile:         opts.TLSCACert,
		})

		if err != nil {
			return &k8shelm.Client{}, err
		}
		options = append(options, k8shelm.WithTLS(tlscfg))
	}

	return k8shelm.NewClient(options...), nil
}

func ClientSetup(logger log.Logger, kubeClient *kubernetes.Clientset, tillerOpts TillerOptions) *k8shelm.Client {
	var helmClient *k8shelm.Client
	var err error
	for {
		helmClient, err = newClient(kubeClient, tillerOpts)
		if err != nil {
			logger.Log("error", fmt.Sprintf("Error creating helm client: %v", err))
			time.Sleep(20 * time.Second)
			continue
		}
		logger.Log("info", "Helm client set up")
		break
	}
	return helmClient
}

// GetTillerVersion retrieves tiller version
func GetTillerVersion(cl k8shelm.Client, h string) (string, error) {
	var v *rls.GetVersionResponse
	var err error
	voption := k8shelm.VersionOption(k8shelm.Host(h))
	if v, err = cl.GetVersion(voption); err == nil {
		return "", fmt.Errorf("error getting tiller version: %v", err)
	}

	return v.GetVersion().String(), nil
}

// TODO ... set up based on the tiller existing in the cluster, if no ops given
func tillerHost(kubeClient *kubernetes.Clientset, opts TillerOptions) (string, error) {
	if opts.Host == "" || opts.Port == "" {
		ts, err := kubeClient.CoreV1().Services(opts.Namespace).Get("tiller-deploy", metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s:%v", ts.Name, ts.Namespace, ts.Spec.Ports[0].Port), nil
	}

	return fmt.Sprintf("%s:%s", opts.Host, opts.Port), nil
}
