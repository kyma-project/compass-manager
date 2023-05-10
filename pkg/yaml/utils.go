package yaml

import (
	kyma "github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"io"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func LoadData(r io.Reader) ([]kyma.Kyma, error) {
	results := make([]kyma.Kyma, 0)
	decoder := yaml.NewDecoder(r)

	for {
		var obj map[string]interface{}
		err := decoder.Decode(&obj)

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		u := unstructured.Unstructured{Object: obj}
		if u.GetObjectKind().GroupVersionKind().Kind == "CustomResourceDefinition" {
			gvk := u.GroupVersionKind()
			kymas := kyma.Kyma{}
			kymas.SetGroupVersionKind(gvk)
			results = append(results, kymas)
			continue
		}
	}

	return results, nil
}
