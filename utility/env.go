package utility

import "os"

const (
	UnitTest           = "Unit Test"
	IntegrationTest    = "Integration Test"
	UtContext          = "quality-gate-ut"
	IntegrationContext = "quality-gate-integration"
)

var (
	GithubToken                = ""
	RepoOwner                  = ""
	RepoName                   = ""
	PrNumber                   = ""
	TestType                   = ""
	CommitID                   = ""
	BranchName                 = ""
	TotalTests                 = ""
	ExcludedCodeFiles          = ""
	ServiceConfigs             ServiceConfig
	GithubActionId             = ""
	ArgoWorkflowNameSpaceAndId = ""
)

func InitEnvVars() {
	GithubToken = os.Getenv("GITHUB_TOKEN")
	RepoOwner = os.Getenv("REPO_OWNER")
	RepoName = os.Getenv("REPO_NAME")
	PrNumber = os.Getenv("PR_NUMBER")
	TestType = os.Getenv("TEST_TYPE")
	CommitID = os.Getenv("COMMIT_ID")
	BranchName = os.Getenv("BRANCH_NAME")
	TotalTests = os.Getenv("TOTAL_TESTS")
	ExcludedCodeFiles = os.Getenv("EXCLUDED_CODE_FILES")

	InitConfigValues()
}
