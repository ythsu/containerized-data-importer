package system

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestPrLimit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Prlimit Suite", reporters.NewReporters())
}
