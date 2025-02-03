package names

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/equinor/radix-common/utils"
)

var defaultSrc = rand.NewSource(time.Now().UnixNano())

// NewRadixBatchJobName create a job name
func NewRadixBatchJobName() string {
	return strings.ToLower(utils.RandStringSeed(8, defaultSrc))
}

// NewRadixBatchName Generate batch name
func NewRadixBatchName(jobComponentName string) string {
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("batch-%s-%s-%s", getJobComponentNamePart(jobComponentName), timestamp, strings.ToLower(utils.RandString(8)))
}

func getJobComponentNamePart(jobComponentName string) string {
	componentNamePart := jobComponentName
	if len(componentNamePart) > 12 {
		componentNamePart = componentNamePart[:12]
	}
	return fmt.Sprintf("%s%s", componentNamePart, strings.ToLower(utils.RandString(16-len(componentNamePart))))
}
