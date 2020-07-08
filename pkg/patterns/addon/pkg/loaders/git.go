package loaders

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
	"io/ioutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

type GitRepository struct {
	baseURL string
}

var _ Repository = &GitRepository{}

// NewGitRepository constructs an GitRepository
func NewGitRepository(baseurl string) *GitRepository{
	return &GitRepository{baseURL: baseurl}
}

func (r *GitRepository) LoadChannel(ctx context.Context,  name string) (*Channel, error){
	if !allowedChannelName(name) {
		return nil, fmt.Errorf("invalid channel name: %q", name)
	}

	log := log.Log
	log.WithValues("baseURL", r.baseURL).Info("loading channel")
	log.WithValues("baseURL", r.baseURL).Info("cloning git repository")

	b, err := r.readURL(name)
	if err != nil{
		log.WithValues("path", name).Error(err, "error reading channel")
		return nil, err
	}
	fmt.Println(string(b))

	channel := &Channel{}
	if err := yaml.Unmarshal(b, channel); err != nil {
		return nil, fmt.Errorf("error parsing channel %s: %v", name, err)
	}

	return channel, nil
}


func (r *GitRepository) LoadManifest(ctx context.Context, packageName string, id string) (map[string]string, error){
	if !allowedManifestId(packageName) {
		return nil, fmt.Errorf("invalid package name: %q", id)
	}

	if !allowedManifestId(id) {
		return nil, fmt.Errorf("invalid manifest id: %q", id)
	}

	log := log.Log
	log.WithValues("package", packageName).Info("loading package")

	filePath := fmt.Sprintf("packages/%v/%v/manifest.yaml", packageName, id)
	fullPath := fmt.Sprintf("%v/%v", r.baseURL, filePath)
	fmt.Println(fullPath)
	b, err := r.readURL(filePath)

	if err != nil {
		return nil, fmt.Errorf("error reading package %s: %v", filePath, err)
	}
	result := map[string]string {
		filePath: string(b),
	}

	return result,nil
}

func (r *GitRepository) readURL(url string) ([]byte, error) {
	fs := memfs.New()
	_, err := git.Clone(memory.NewStorage(), fs, &git.CloneOptions{
		URL: r.baseURL,
	})
	if err != nil {
		return nil, err
	}

	file, err := fs.Open(url)
	if err != nil{
		return nil, err
	}

	b, err := ioutil.ReadAll(file)
	if err != nil{
		return nil, err
	}

	return b, nil
}
