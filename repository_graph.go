package cibudgetdomain

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

func (repository *Repository) deriveGraph() error {
	graph := Graph{SourceDigest: repository.Digest}
	if graph.SourceDigest == "" {
		graph.SourceDigest = "UNSEALED_OBSERVATION"
	}
	repository.Commands = map[string][]string{}
	repository.GlobalInputChanges = nil
	indices := map[string]int{}
	owners := map[string][]int{}
	for _, pkg := range repository.Packages {
		if pkg.ImportPath == "" || strings.HasPrefix(pkg.ImportPath, "-") {
			return fmt.Errorf("package identity differs")
		}
		if _, exists := indices[pkg.ImportPath]; exists {
			return fmt.Errorf("package identity is repeated")
		}
		indices[pkg.ImportPath] = len(graph.Units)
		graph.Units = append(graph.Units, Unit{ID: "package:" + pkg.ImportPath, Kind: "source"}, Unit{ID: "test-source:" + pkg.ImportPath, Kind: "source"}, Unit{ID: "test:" + pkg.ImportPath, Kind: "test"})
	}
	for _, pkg := range repository.Packages {
		index := indices[pkg.ImportPath]
		graph.Units[index].Dependencies = localImports(pkg.Imports, indices)
		dependencies := append([]int{index}, localImports(append(append([]string{}, pkg.TestImports...), pkg.XTestImports...), indices)...)
		graph.Units[index+1].Dependencies = uniqueIndices(dependencies)
		graph.Units[index+2].Dependencies = []int{index, index + 1}
		args := []string{"test", "-mod=readonly", "-count=1", "-p=1", "-vet=off", "-json", pkg.ImportPath, "-args", "-test.testlogfile={{origin_file}}"}
		repository.Commands[graph.Units[index+2].ID] = args
		graph.Units[index+2].CostKey = hashBytes([]byte(repository.Profile + "\x00" + strings.Join(args, "\x00")))
		production := [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.EmbedFiles}
		for _, group := range production {
			for _, name := range group {
				owners[path.Join(pkg.Dir, name)] = append(owners[path.Join(pkg.Dir, name)], index)
			}
		}
		for _, group := range [][]string{pkg.TestGoFiles, pkg.XTestGoFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles} {
			for _, name := range group {
				owners[path.Join(pkg.Dir, name)] = append(owners[path.Join(pkg.Dir, name)], index+1)
			}
		}
	}
	globalHub := -1
	seen := map[string]bool{}
	for _, changed := range repository.Changes {
		if changed == "" || changed != path.Clean(changed) || path.IsAbs(changed) || changed == ".." || strings.HasPrefix(changed, "../") || seen[changed] {
			return fmt.Errorf("changed input identity differs")
		}
		seen[changed] = true
		input := len(graph.Units)
		graph.Units = append(graph.Units, Unit{ID: "input:" + changed, Kind: "source"})
		graph.Changed = append(graph.Changed, input)
		consumers := uniqueIndices(owners[changed])
		if len(consumers) == 0 {
			repository.GlobalInputChanges = append(repository.GlobalInputChanges, changed)
			if globalHub < 0 {
				globalHub = len(graph.Units)
				graph.Units = append(graph.Units, Unit{ID: "global-tracked-input-domain", Kind: "source"})
				for _, pkg := range repository.Packages {
					testIndex := indices[pkg.ImportPath] + 2
					graph.Units[testIndex].Dependencies = append(graph.Units[testIndex].Dependencies, globalHub)
				}
			}
			consumers = []int{globalHub}
		}
		for _, consumer := range consumers {
			graph.Units[consumer].Dependencies = append(graph.Units[consumer].Dependencies, input)
		}
	}
	repository.Graph = graph
	_, err := validateGraph(graph)
	return err
}

func localImports(imports []string, indices map[string]int) []int {
	result := []int{}
	for _, dependency := range imports {
		if index, ok := indices[dependency]; ok {
			result = append(result, index)
		}
	}
	return uniqueIndices(result)
}
func uniqueIndices(values []int) []int {
	sort.Ints(values)
	result := []int{}
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
