package collector

import (
	"fmt"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	"helm.sh/helm/pkg/storage"
	"helm.sh/helm/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/releaseutil"

	"github.com/ghodss/yaml"
)

type HelmV2Collector struct {
	*commonCollector
	client       *corev1.CoreV1Client
	secretsStore *storage.Storage
	configStore  *storage.Storage
}

type HelmV2Opts struct {
	Kubeconfig string
}

func NewHelmV2Collector(opts *HelmV2Opts) (*HelmV2Collector, error) {
	collector := &HelmV2Collector{commonCollector: &commonCollector{name: "Helm v2"}}

	config, err := clientcmd.BuildConfigFromFlags("", opts.Kubeconfig)
	if err != nil {
		return nil, err
	}

	collector.client, err = corev1.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	secretsDriver := driver.NewSecrets(collector.client.Secrets(""))
	collector.secretsStore = storage.Init(secretsDriver)

	configDriver := driver.NewConfigMaps(collector.client.ConfigMaps(""))
	collector.configStore = storage.Init(configDriver)

	return collector, nil
}

func (c *HelmV2Collector) Get() ([]map[string]interface{}, error) {

	releases, err := c.secretsStore.ListDeployed()
	if err != nil {
		return nil, err
	}

	releasesConfig, err := c.configStore.ListDeployed()
	if err != nil {
		return nil, err
	}

	releases = append(releases, releasesConfig...)

	var results []map[string]interface{}

	for _, r := range releases {
		manifests := releaseutil.SplitManifests(r.Manifest)
		for _, m := range manifests {
			var manifest map[string]interface{}

			err := yaml.Unmarshal([]byte(m), &manifest)
			if err != nil {
				err := fmt.Errorf("failed to parse release %s/%s: %v", r.Namespace, r.Name, err)
				return nil, err
			}

			// Default to the release namespace if the manifest doesn't have the namespace set
			if meta, ok := manifest["metadata"]; ok {
				switch v := meta.(type) {
				case map[string]interface{}:
					if _, ok := v["namespace"]; !ok {
						v["namespace"] = r.Namespace
					}
				}
			}

			results = append(results, manifest)
		}
	}

	return results, nil
}
