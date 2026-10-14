package cibudgetdomain

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func unitCostKey(repository Repository, id string) string {
	for _, unit := range repository.Graph.Units {
		if unit.ID == id {
			return unit.CostKey
		}
	}
	return ""
}

// This is an observation of testing's os API hook. It reports a bounded set of
// origins; it makes no completeness claim about subprocesses or direct syscalls.
func observedTestInputs(repository Repository, root, id, name string) ([]InputOrigin, string) {
	var pkg *PackageInput
	for index := range repository.Packages {
		if "test:"+repository.Packages[index].ImportPath == id {
			pkg = &repository.Packages[index]
			break
		}
	}
	if pkg == nil {
		return nil, "UNBOUND_PACKAGE"
	}
	file, err := os.Open(name)
	if err != nil {
		if len(pkg.TestGoFiles)+len(pkg.XTestGoFiles) == 0 {
			return nil, "NO_TEST_BINARY"
		}
		return nil, "UNOBSERVED"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 4*1024*1024 {
		return nil, "INPUT_LOG_LIMIT"
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "UNBOUND_SOURCE_ROOT"
	}
	cwd := filepath.Join(root, filepath.FromSlash(pkg.Dir))
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 65536)
	if !scanner.Scan() || scanner.Text() != "# test log" {
		return nil, "UNRECOGNIZED_TOOLCHAIN_LOG"
	}
	result := []InputOrigin{}
	seen := map[string]bool{}
	for scanner.Scan() {
		operation, value, ok := strings.Cut(scanner.Text(), " ")
		if !ok {
			return result, "PARTIAL"
		}
		if operation == "getenv" {
			continue
		} // Environment values are never collected.
		if operation != "open" && operation != "stat" && operation != "chdir" {
			return result, "PARTIAL"
		}
		absolute := value
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(cwd, value)
		}
		absolute = filepath.Clean(absolute)
		relative, err := filepath.Rel(root, absolute)
		if err != nil {
			return result, "PARTIAL"
		}
		observed := filepath.ToSlash(relative)
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			observed = "external:" + hashBytes([]byte(absolute))
		}
		key := operation + "\x00" + observed
		if !seen[key] {
			if len(result) >= 512 {
				return result, "INPUT_POPULATION_LIMIT"
			}
			result = append(result, InputOrigin{operation, observed})
			seen[key] = true
		}
		if operation == "chdir" {
			cwd = absolute
		}
	}
	if scanner.Err() != nil {
		return result, "PARTIAL"
	}
	return result, "OBSERVED_OS_API"
}
