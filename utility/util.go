package utility

import (
	"bufio"
	"errors"
	"fmt"
	"go-patch-cover/config"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type CoverageData struct {
	ServiceCoverage string
	PatchCoverage   string
	Statements      string
	NumStmt         int
}

type ServiceConfig struct {
	UTServiceThreshold           float64 `mapstructure:"ut_service_threshold"`
	UTMCCThreshold               float64 `mapstructure:"ut_mcc_threshold"`
	IntegrationServiceThreshold  float64 `mapstructure:"integration_service_threshold"`
	IntegrationMCCThreshold      float64 `mapstructure:"integration_mcc_threshold"`
	ExcludedUTCodeFiles          string  `mapstructure:"excluded_ut_code_files"`
	ExcludedIntegrationCodeFiles string  `mapstructure:"excluded_integration_code_files"`
}

func ParseCoverageInfo(filePath string) (CoverageData, error) {
	coverageData, err := readCoverageData(filePath)
	if err != nil {
		fmt.Println("Error:", err)
		return CoverageData{}, errors.New("issue with parsing the file")
	}

	return coverageData, nil
}

func readCoverageData(filePath string) (CoverageData, error) {
	var data CoverageData

	file, err := os.Open(filePath)
	if err != nil {
		return data, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "new coverage:") {
			data.ServiceCoverage = parseCoverage(line)
		} else if strings.HasPrefix(line, "patch coverage:") {
			data.PatchCoverage = parseCoverage(line)
			data.Statements = parseStatements(line)
		} else if strings.HasPrefix(line, "total") {
			data.NumStmt = parseTotalLines(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return data, err
	}

	return data, nil
}

func parseCoverage(line string) string {
	re := regexp.MustCompile(`\d+\.\d+%`)
	match := re.FindString(line)
	if match != "" {
		return strings.TrimRight(match, "%")
	}
	return match
}

func parseStatements(line string) string {
	re := regexp.MustCompile(`\(\d+/\d+\)`)
	match := re.FindString(line)
	if match != "" {
		return strings.Trim(match, "()")
	}
	return match
}

func parseTotalLines(line string) int {
	re := regexp.MustCompile(`total (\d+) statements`)
	match := re.FindStringSubmatch(line)
	if len(match) < 2 {
		return 0
	}
	numStr := match[1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		fmt.Println("Error converting to integer:", err)
		return 0
	}
	return num
}

func IsMaster() bool {
	return BranchName == "master" || BranchName == "main"
}

// InitConfigValues fetches the thresholds and exclusion files based on the service
func InitConfigValues() {
	var fileName string
	switch RepoName {
	case "":
		fileName = config.DefaultFileName
	default:
		fileName = RepoName
	}

	viper.SetConfigName(fileName)
	viper.SetConfigType(config.DefaultFileTye)
	viper.AddConfigPath(config.DefaultConfigDir)

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("config file not set for %s service, using %s.%s\n", RepoName, config.DefaultFileName, config.DefaultFileTye)
		viper.SetConfigName(config.DefaultFileName)
		if err := viper.ReadInConfig(); err != nil {
			fmt.Printf("Error reading %s.%s", config.DefaultFileName, config.DefaultFileTye)
		}
	} else {
		fmt.Printf("reading config values from %s.yaml\n", RepoName)
	}

	// Unmarshal the configuration into the struct
	if err := viper.Unmarshal(&ServiceConfigs); err != nil {
		fmt.Printf("Error unmarshaling config: %v", err)
	}

}

// GetExcludedCodeFile Logic: if env variable is present, return that
// else if <service_name>.yaml is present, return from that. else return from default.yaml
func GetExcludedCodeFile() string {
	if isExcludedCodeFileOverridden() {
		fmt.Println("Exclude code files parameters sent, overriding the config values!")
		return ExcludedCodeFiles
	}

	switch TestType {
	case UnitTest:
		return ServiceConfigs.ExcludedUTCodeFiles
	default:
		return ServiceConfigs.ExcludedIntegrationCodeFiles
	}
}

func isExcludedCodeFileOverridden() bool {
	if ExcludedCodeFiles == "" {
		return false
	}
	return !strings.Contains(ExcludedCodeFiles, "=jsonpath")
}

// GetThresholdConfigBasedOnTestType get config value based on test type
func GetThresholdConfigBasedOnTestType() (float64, float64) {
	var (
		serviceThreshold float64
		mccThreshold     float64
	)

	switch TestType {
	case UnitTest:
		serviceThreshold, mccThreshold = ServiceConfigs.UTServiceThreshold, ServiceConfigs.UTMCCThreshold
	case IntegrationTest:
		serviceThreshold, mccThreshold = ServiceConfigs.IntegrationServiceThreshold, ServiceConfigs.IntegrationMCCThreshold
	}
	return serviceThreshold, mccThreshold
}

// GetThresholdCondition Condition to determine whether service coverage and patch coverage meet the specified thresholds
func GetThresholdCondition(serviceCoverage float64, mccCoverage float64) (bool, bool) {
	var scs, mccs bool

	switch TestType {
	case UnitTest:
		scs = serviceCoverage >= ServiceConfigs.UTServiceThreshold
		mccs = mccCoverage >= ServiceConfigs.UTMCCThreshold
	case IntegrationTest:
		scs = serviceCoverage >= ServiceConfigs.IntegrationServiceThreshold
		mccs = mccCoverage >= ServiceConfigs.IntegrationMCCThreshold
	}

	return scs, mccs
}

// GetCoverageDetails Condition for parsing the coverage file and extracting the service coverage and patch coverage numbers
func GetCoverageDetails(filePath string) (float64, float64) {
	cd, err := ParseCoverageInfo(filePath)
	if err != nil {
		fmt.Printf("Error parsing coverage info: %v\n", err)
		return 0, 0
	}

	serviceCoverage, err := strconv.ParseFloat(cd.ServiceCoverage, 64)
	if err != nil {
		fmt.Printf("Error parsing service coverage: %v\n", err)
		return 0, 0
	}

	patchCoverage, err := strconv.ParseFloat(cd.PatchCoverage, 64)
	if err != nil {
		fmt.Printf("Error parsing patch coverage: %v\n", err)
		return 0, 0
	}

	return serviceCoverage, patchCoverage
}
