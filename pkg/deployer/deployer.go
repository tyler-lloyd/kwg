package deployer

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	kubeSystemNS             = "kube-system"
	wireguardSecretName      = "wireguard"
	serverConfigTemplateFile = "wg0.conf.template"
)

type deployer struct {
	k8sClient *kubernetes.Clientset
	server    wgtypes.Config
}

func New(cs *kubernetes.Clientset) *deployer {
	return &deployer{
		k8sClient: cs,
	}
}

func (d *deployer) DeployServer(ctx context.Context) error {
	// ensure secret
	d.ensureConfigSecret(ctx)

	// ensure deployment
	d.ensureDeployment(ctx)

	// ensure service
	d.ensureService(ctx)
	return nil
}

func (d *deployer) ensureService(ctx context.Context) {
	_, err := d.k8sClient.CoreV1().Services(kubeSystemNS).Get(ctx, wireguardSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		panic(err)
	}

	if err == nil {
		return
	}

	wgService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wireguardSecretName,
			Namespace: kubeSystemNS,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"name": wireguardSecretName,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolUDP,
					Port:     int32(51820),
				},
			},
		},
	}

	s, err := d.k8sClient.CoreV1().Services(kubeSystemNS).Create(ctx, wgService, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("service %s created\n", s.Name)
}

func (d *deployer) ensureDeployment(ctx context.Context) {
	_, err := d.k8sClient.AppsV1().Deployments(kubeSystemNS).Get(ctx, wireguardSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		panic(err)
	}

	if err == nil {
		// already there nothing to do
		return
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode([]byte(serverDeployment), nil, nil)
	if err != nil {
		panic(err)
	}

	deployment := obj.(*appsv1.Deployment)
	deployment.Name = wireguardSecretName
	deployment.Namespace = kubeSystemNS
	deployment.Spec.Replicas = to.Ptr(int32(2))
	_, err = d.k8sClient.AppsV1().Deployments(kubeSystemNS).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}

func (d *deployer) ensureConfigSecret(ctx context.Context) {
	_, err := d.k8sClient.CoreV1().Secrets(kubeSystemNS).Get(ctx, wireguardSecretName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		panic(err)
	}

	if err == nil {
		return
	}
	// create secret
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		panic(err)
	}

	// inject key
	serverConfig := strings.ReplaceAll(serverConfigTemplate, "{PRIVATE_KEY}", key.String())

	// create
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wireguardSecretName,
			Namespace: kubeSystemNS,
			Annotations: map[string]string{
				"azure.kubernetes.com/publickey": key.PublicKey().String(),
			},
		},
		StringData: map[string]string{
			serverConfigTemplateFile: serverConfig,
		},
	}

	_, err = d.k8sClient.CoreV1().Secrets(kubeSystemNS).Create(ctx, s, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("server can be reached at %s\n", key.PublicKey())
	// set the server config
	d.server.PrivateKey = &key
}
