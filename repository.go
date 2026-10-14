package cibudgetdomain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type PackageInput struct {
	ImportPath, Dir                                                                              string
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles []string
	TestGoFiles, XTestGoFiles, EmbedFiles, TestEmbedFiles, XTestEmbedFiles                       []string
	Imports, TestImports, XTestImports                                                           []string
	Incomplete                                                                                   bool
	Error                                                                                        *struct{ Err string }
}

type Repository struct {
	Schema, Base, Head, Tree, Module, Profile, Digest string
	Packages                                          []PackageInput
	Changes                                           []string
	Graph                                             Graph
	Commands                                          map[string][]string
	GlobalInputChanges                                []string
	Workers                                           int
	ScopeStartedAt                                    time.Time
	PolicyDigest                                      string
	CollectionMS                                      int64
}

// Collect observes the host module's tracked-source change domain. Unowned
// inputs fan out to every test package; they are never silently excluded.
func Collect(ctx context.Context, root, base, head string) (Repository, error) {
	return CollectWithWorkers(ctx, root, base, head, 1)
}

func CollectWithWorkers(ctx context.Context, root, base, head string, workers int) (Repository, error) {
	start := time.Now()
	result := Repository{Schema: "gooo/ci-tracked-source-domain/v1", Workers: workers, ScopeStartedAt: start, PolicyDigest: SourcePolicyDigest()}
	if workers < 1 || workers > 64 {
		return result, fmt.Errorf("worker capacity requires 1..64")
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return result, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return result, err
	}
	if result.Base, err = resolveCommit(ctx, root, base); err != nil {
		return result, err
	}
	if result.Head, err = resolveCommit(ctx, root, head); err != nil {
		return result, err
	}
	if err = verifyCleanHead(ctx, root, result.Head); err != nil {
		return result, err
	}
	tree, err := commandOutput(ctx, root, "git", []string{"rev-parse", result.Head + "^{tree}"})
	if err != nil {
		return result, err
	}
	result.Tree = strings.TrimSpace(string(tree))
	env, err := commandOutput(ctx, root, "go", []string{"env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT", "GOTOOLCHAIN", "GOWORK"})
	if err != nil {
		return result, err
	}
	result.Profile = hashBytes(append(env, []byte(fmt.Sprintf("\n-p=1;-count=1;-vet=off;ambient-build-cache;GOMAXPROCS=2;workers=%d;logical_cpu=%d", workers, runtime.NumCPU()))...))
	list, err := commandOutput(ctx, root, "go", []string{"list", "-mod=readonly", "-e", "-json", "./..."})
	if err != nil {
		return result, err
	}
	if result.Packages, err = decodePackages(list, root); err != nil {
		return result, err
	}
	mod, err := commandOutput(ctx, root, "go", []string{"list", "-m"})
	if err != nil {
		return result, err
	}
	result.Module = strings.TrimSpace(string(mod))
	changed, err := commandOutput(ctx, root, "git", []string{"diff", "--name-only", "-z", result.Base, result.Head, "--"})
	if err != nil {
		return result, err
	}
	for _, path := range strings.Split(string(changed), "\x00") {
		if path != "" {
			result.Changes = append(result.Changes, path)
		}
	}
	if len(result.Changes) == 0 {
		return result, fmt.Errorf("source revisions have no changed tracked inputs")
	}
	if err = result.deriveGraph(); err != nil {
		return result, err
	}
	result.CollectionMS = time.Since(start).Milliseconds()
	result.Digest = result.sourceDigest()
	result.Graph.SourceDigest = result.Digest
	return result, nil
}

func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func (repository Repository) sourceDigest() string {
	repository.ScopeStartedAt = time.Time{}
	repository.CollectionMS = 0
	repository.Digest = ""
	repository.Graph.SourceDigest = ""
	data, _ := json.Marshal(repository)
	return hashBytes(data)
}

func (repository Repository) Validate() error {
	if repository.Schema != "gooo/ci-tracked-source-domain/v1" || repository.PolicyDigest != SourcePolicyDigest() || repository.Digest != repository.sourceDigest() ||
		repository.Graph.SourceDigest != repository.Digest {
		return fmt.Errorf("repository projection digest differs")
	}
	expected := repository
	if err := expected.deriveGraph(); err != nil {
		return err
	}
	expected.Graph.SourceDigest = repository.Digest
	a, _ := json.Marshal(expected.Graph)
	b, _ := json.Marshal(repository.Graph)
	c, _ := json.Marshal(expected.Commands)
	d, _ := json.Marshal(repository.Commands)
	if string(a) != string(b) || string(c) != string(d) {
		return fmt.Errorf("graph or command differs from observed package inputs")
	}
	return nil
}

func decodePackages(data []byte, root string) ([]PackageInput, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	packages := []PackageInput{}
	for decoder.More() {
		var unit PackageInput
		if err := decoder.Decode(&unit); err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(root, unit.Dir)
		if err != nil || relative == ".." || strings.HasPrefix(relative, "../") {
			return nil, fmt.Errorf("package is outside module source")
		}
		unit.Dir = filepath.ToSlash(relative)
		if unit.Incomplete || unit.Error != nil {
			return nil, fmt.Errorf("package import observations are incomplete: %s", unit.ImportPath)
		}
		packages = append(packages, unit)
	}
	if len(packages) == 0 || len(packages) > 1024 {
		return nil, fmt.Errorf("package population requires 1..1024")
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].ImportPath < packages[j].ImportPath })
	return packages, nil
}
