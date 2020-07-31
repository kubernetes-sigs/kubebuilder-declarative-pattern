package loaders

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"io/ioutil"
	"path"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
	"strings"
)

type GitRepository struct {
	baseURL string
	subDir string
	branch string
}

var _ Repository = &GitRepository{}

// NewGitRepository constructs an GitRepository
func NewGitRepository(baseurl string) *GitRepository{
	repo := parseGitURL(baseurl)
	return &repo
}

func (r *GitRepository) LoadChannel(ctx context.Context,  name string) (*Channel, error){
	if !allowedChannelName(name) {
		return nil, fmt.Errorf("invalid channel name: %q", name)
	}

	log := log.Log
	log.WithValues("baseURL", r.baseURL).Info("loading channel")
	log.WithValues("baseURL", r.baseURL).Info("cloning git repository")

	if r.subDir != "" {
		name = r.subDir + "/" + name
	}
	b, err := r.readURL(name)
	if err != nil{
		log.WithValues("path", name).Error(err, "error reading channel")
		return nil, err
	}

	channel := &Channel{}
	if err := yaml.Unmarshal(b, channel); err != nil {
		return nil, fmt.Errorf("error parsing channel bytes %s: %v", string(b), err)
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

	var filePath string
	if r.subDir == "" {
		filePath = path.Join("packages", packageName, id,"manifest.yaml" )
	} else {
		filePath = path.Join(r.subDir,"packages", packageName, id, "manifest.yaml")
	}

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
	repoDir := "/tmp/repo"
	filePath := filepath.Join(repoDir, url)
	fmt.Println(r.baseURL)
	_, err := git.PlainClone(repoDir, false, &git.CloneOptions{
		URL:               r.baseURL,
	})
	if err != nil && err != git.ErrRepositoryAlreadyExists {
		return nil, err
	}

	b, err := ioutil.ReadFile(filePath)
	if err != nil{
		return nil, err
	}

	return b, nil
}

func parseGitURL(url string) GitRepository{
	// checks for git:: suffix
	var subdir string
	if strings.HasPrefix(url, "git::") {
		url = strings.TrimPrefix(url, "git::")
	}

	// checks for subdirectories
	if strings.Contains(url, ".git//") {
		urlComponent := strings.SplitN(url, ".git//", 2)
		url = urlComponent[0] + ".git"
		subdir = urlComponent[1]
	}

	return GitRepository{
		baseURL: url,
		subDir:  subdir,
	}
}
