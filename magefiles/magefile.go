//go:build mage
// +build mage

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/bitfield/script"
	"github.com/common-fate/common-fate/pkg/deploy"
	"github.com/common-fate/common-fate/pkg/identity/identitysync"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
	"go.uber.org/zap"
)

func init() {
	// setup logging
	logCfg := zap.NewDevelopmentConfig()
	logCfg.DisableStacktrace = true
	log, err := logCfg.Build()
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
	zap.ReplaceGlobals(log)
	_ = godotenv.Load()
}

// ldFlags returns the linker flags. These are used to inject the release details into the
// built binaries.
func ldFlags() string {
	flags := []string{}
	release := os.Getenv("COMMONFATE_RELEASE")
	apiURL := os.Getenv("COMMONFATE_PROVIDER_REGISTRY_API_URL")
	if release == "" {
		release = "dev"
	} else if apiURL == "" {
		// commonfate_release will be populated for a production build, using it here to trigger setting teh registry url to the production registry
		flags = append(flags, `-X 'github.com/common-fate/common-fate/internal/build.ProviderRegistryAPIURL=https://api.registry.commonfate.io'`)
	} else {
		// Allows a deve deployment to be configured to use a custom API URL
		flags = append(flags, fmt.Sprintf(`-X 'github.com/common-fate/common-fate/internal/build.ProviderRegistryAPIURL=%s'`, apiURL))
	}

	flags = append(flags, fmt.Sprintf(`-X 'github.com/common-fate/common-fate/internal/build.Version=%s'`, release))
	return strings.Join(flags, " ")
}

type Deps mg.Namespace

// NPM installs NPM dependencies for the repository using pnpm.
func (Deps) NPM() error {
	_, err := os.Stat("node_modules")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		fmt.Println("node_modules already exists, skipping installing dependencies")
		return nil
	}

	fmt.Println("node_modules not found: installing with pnpm")
	return sh.Run("pnpm", "install")
}

type Build mg.Namespace

// Backend builds the Go API for the AWS Lambda runtime.
func (Build) Backend() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/commonfate", "cmd/lambda/commonfate/handler.go")
}

func (Build) FrontendDeployer() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/frontend-deployer", "cmd/lambda/frontend-deployer/handler.go")
}

func (Build) Syncer() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/syncer", "cmd/lambda/syncer/handler.go")
}
func (Build) HealthChecker() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/healthcheck", "cmd/lambda/healthcheck/handler.go")
}
func (Build) CacheSyncer() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/cache-sync", "cmd/lambda/cache-sync/handler.go")
}

func (Build) SlackNotifier() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/slack-notifier", "cmd/lambda/event-handlers/notifiers/slack/handler.go")
}

func (Build) Webhook() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/webhook", "cmd/lambda/webhook/handler.go")
}
func (Build) TargetGroupGranter() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/targetgroup-granter", "cmd/lambda/targetgroup-granter/handler.go")
}
func (Build) Governance() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/governance", "cmd/lambda/governance/handler.go")
}

func (Build) EventHandler() error {
	env := map[string]string{
		"GOOS":   "linux",
		"GOARCH": "amd64",
	}
	return sh.RunWith(env, "go", "build", "-ldflags", ldFlags(), "-o", "bin/event-handler", "cmd/lambda/event-handlers/eventhandler/handler.go")
}
func (Build) FrontendAWSExports() error {
	// create the aws-exports.js file if it doesn't exist. This prevents the frontend build from breaking.
	f := "web/src/utils/aws-exports.js"
	if _, err := os.Stat(f); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("%s doesnt exist: creating a placeholder file\n", f)
		_, err = script.Echo("export default {};").WriteFile(f)
		return err
	}
	return nil
}

// Frontend generates the React static frontend.
func (Build) Frontend() error {
	mg.Deps(Deps.NPM, Build.FrontendAWSExports)
	dirs := []string{"web/*"}

	files, err := os.ReadDir("web")
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() && f.Name() != "node_modules" && f.Name() != ".next" {
			dirs = append(dirs, path.Join("web", f.Name()))
		}
	}

	// don't rebuild unless any React source files have changed
	changed, err := target.Glob("web/dist", dirs...)
	if err != nil {
		return err
	}

	if !changed {
		fmt.Println("skipping frontend build: no frontend files changed")
		return nil
	}

	fmt.Println("building frontend...")
	return sh.Run("pnpm", "--dir", "web", "build")
}

// PackageBackend zips the Go API so that it can be deployed to Lambda.
func PackageBackend() error {
	mg.Deps(Build.Backend)
	return sh.Run("zip", "--junk-paths", "bin/commonfate.zip", "bin/commonfate")
}

// PackageTargetGroupGranter zips the Go TargetGroupGranter so that it can be deployed to Lambda.
func PackageTargetGroupGranter() error {
	mg.Deps(Build.TargetGroupGranter)
	return sh.Run("zip", "--junk-paths", "bin/targetgroup-granter.zip", "bin/targetgroup-granter")
}

// PackageHealthChecker zips the Go deployment health checker so that it can be deployed to Lambda.
func PackageHealthChecker() error {
	mg.Deps(Build.HealthChecker)
	return sh.Run("zip", "--junk-paths", "bin/healthcheck.zip", "bin/healthcheck")
}

func Package() {
	mg.Deps(PackageBackend, PackageSlackNotifier, PackageEventHandler)
	mg.Deps(PackageSyncer, PackageWebhook, PackageGovernance, PackageFrontendDeployer)
	mg.Deps(PackageCacheSyncer, PackageHealthChecker, PackageTargetGroupGranter)
}

// PackageFrontendDeployer zips the Go frontend deployer so that it can be deployed to Lambda.
func PackageFrontendDeployer() error {
	mg.Deps(Build.FrontendDeployer)
	return sh.Run("zip", "--junk-paths", "bin/frontend-deployer.zip", "bin/frontend-deployer")
}

// PackageSyncer zips the Go Syncer function handler so that it can be deployed to Lambda.
func PackageSyncer() error {
	mg.Deps(Build.Syncer)
	return sh.Run("zip", "--junk-paths", "bin/syncer.zip", "bin/syncer")
}

// PackageSyncer zips the Go Syncer function handler so that it can be deployed to Lambda.
func PackageCacheSyncer() error {
	mg.Deps(Build.CacheSyncer)
	return sh.Run("zip", "--junk-paths", "bin/cache-sync.zip", "bin/cache-sync")
}

// PackageNotifier zips the Go notifier so that it can be deployed to Lambda.
func PackageSlackNotifier() error {
	mg.Deps(Build.SlackNotifier)
	return sh.Run("zip", "--junk-paths", "bin/slack-notifier.zip", "bin/slack-notifier")
}

// PackageWebhook zips the Go webhook handler so that it can be deployed to Lambda.
func PackageWebhook() error {
	mg.Deps(Build.Webhook)
	return sh.Run("zip", "--junk-paths", "bin/webhook.zip", "bin/webhook")
}

// PackageGovernance zips the Go governance handler so that it can be deployed to Lambda.
func PackageGovernance() error {
	mg.Deps(Build.Governance)
	return sh.Run("zip", "--junk-paths", "bin/governance.zip", "bin/governance")
}

// PackageEventHandler zips the Go event handler so that it can be deployed to Lambda.
func PackageEventHandler() error {
	mg.Deps(Build.EventHandler)
	return sh.Run("zip", "--junk-paths", "bin/event-handler.zip", "bin/event-handler")
}

type Deploy mg.Namespace

// CDK deploys the CDK infrastructure stack to AWS
func (Deploy) CDK() error {
	mg.Deps(DevConfig)
	mg.Deps(Deps.NPM)
	// infrastructure/cdk.json defines a build step within CDK which calls `go run mage.go build`,
	// so we don't need to build or package the code before running cdk deploy.
	args := []string{"--dir", "deploy/infra", "cdk", "deploy", "--outputs-file", "cdk-outputs.json"}
	cfg, err := deploy.LoadConfig(deploy.DefaultFilename)
	if err != nil {
		return err
	}
	args = append(args, cfg.CDKContextArgs()...)

	zap.S().Infow("deploying CDK stack", "stack", cfg.Deployment.StackName)

	return sh.Run("pnpm", args...)
}

// Dotenv updates the .env file based on the deployed CDK infrastructure
func AWSExports() error {
	output, err := ensureCDKOutput()
	if err != nil {
		return err
	}
	return output.WriteAWSExports()
}

// Dotenv updates the .env file based on the deployed CDK infrastructure
func Dotenv() error {
	// create a .env file if one doesn't exist.
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		fmt.Println(".env file doesnt exist: copying .env.template to .env")
		err := sh.Run("cp", ".env.template", ".env")
		if err != nil {
			return err
		}
	}

	myEnv, err := godotenv.Read()
	if err != nil {
		return err
	}

	o, err := ensureCDKOutput()
	if err != nil {
		return err
	}

	cfg, err := deploy.LoadConfig(deploy.DefaultFilename)
	if err != nil {
		return err
	}
	idConf := "{}"
	if cfg.Deployment.Parameters.IdentityConfiguration != nil {
		b, err := json.Marshal(cfg.Deployment.Parameters.IdentityConfiguration)
		if err != nil {
			return err
		}
		idConf = string(b)
	}

	idpType := identitysync.IDPTypeCognito
	if cfg.Deployment.Parameters.IdentityProviderType != "" {
		idpType = cfg.Deployment.Parameters.IdentityProviderType
	}

	myEnv["AWS_REGION"] = o.Region
	myEnv["COMMONFATE_TABLE_NAME"] = o.DynamoDBTable
	myEnv["COMMONFATE_COGNITO_USER_POOL_ID"] = o.UserPoolID
	myEnv["COMMONFATE_EVENT_BUS_ARN"] = o.EventBusArn
	myEnv["COMMONFATE_EVENT_BUS_SOURCE"] = o.EventBusSource
	myEnv["COMMONFATE_IDENTITY_SETTINGS"] = idConf
	myEnv["COMMONFATE_IDENTITY_PROVIDER"] = idpType
	myEnv["COMMONFATE_ADMIN_GROUP"] = cfg.Deployment.Parameters.AdministratorGroupID
	myEnv["COMMONFATE_FRONTEND_URL"] = "http://localhost:3000"
	myEnv["COMMONFATE_ACCESS_HANDLER_RUNTIME"] = "lambda"
	myEnv["COMMONFATE_PAGINATION_KMS_KEY_ARN"] = o.PaginationKMSKeyARN
	myEnv["COMMONFATE_ACCESS_REMOTE_CONFIG_URL"] = cfg.Deployment.Parameters.ExperimentalRemoteConfigURL
	myEnv["COMMONFATE_REMOTE_CONFIG_HEADERS"] = cfg.Deployment.Parameters.ExperimentalRemoteConfigHeaders
	myEnv["COMMONFATE_IDENTITY_GROUP_FILTER"] = cfg.Deployment.Parameters.IdentityGroupFilter
	myEnv["COMMONFATE_GRANTER_V2_STATE_MACHINE_ARN"] = o.GranterV2StateMachineArn

	err = godotenv.Write(myEnv, ".env")
	if err != nil {
		return err
	}

	zap.S().Infow("updated .env file with CDK output", "output", o)
	return nil
}

// Frontend uploads the frontend to S3 and invalidates CloudFront
func (Deploy) Frontend() error {
	mg.Deps(Build.Frontend)
	output, err := ensureCDKOutput()
	if err != nil {
		return err
	}

	return output.DeployFrontend()
}

// Dev provisions a development environment
func (Deploy) Dev() error {
	// ensure the user has a valid aws session
	ctx := context.Background()
	_, err := deploy.TryGetCurrentAccountID(ctx)
	if err != nil {
		boldYellow := color.New(color.FgYellow, color.Bold)
		boldYellow.Println("⚠️ Failed to get AWS caller identity, ensure you have a valid AWS session")
		return nil
	}

	// deploy the CDK stack
	mg.Deps(Deploy.CDK)
	// setup the .env file
	mg.Deps(Dotenv)
	// upload the frontend to S3 and invalidate CloudFront
	mg.Deps(Deploy.Frontend)
	return nil
}

// StagingCDK deploys a staging version of the CDK infrastructure.
func (Deploy) StagingCDK(env, name string) error {
	mg.Deps(Deps.NPM)
	boldYellow := color.New(color.FgYellow, color.Bold)
	boldYellow.Printf("creating staging deployment for stage %s in environment %s\n", name, env)

	// infrastructure/cdk.json defines a build step within CDK which calls `go run mage.go build`,
	// so we don't need to build or package the code before running cdk deploy.
	// add a '--require-approval never' arg to avoid being prompted for approval in CI pipelines.
	args := []string{"--dir", "deploy/infra", "cdk", "deploy", "--require-approval", "never", "--outputs-file", "cdk-outputs.json"}

	if os.Getenv("CDK_HOTSWAP") == "true" {
		args = append(args, "--hotswap")
	}

	dep := deploy.NewStagingConfig(context.Background(), name)
	args = append(args, dep.CDKContextArgs()...)
	// add the devEnvironment context arg
	args = append(args, "-c", "devEnvironment="+env)

	zap.S().Infow("deploying CDK stack", "stage", name)

	return sh.Run("pnpm", args...)
}

// StagingFrontend uploads the frontend to the S3 bucket and invalidates CloudFront.
// It requires an internal deployment environment and name to be specified.
func (Deploy) StagingFrontend(env, name string) error {
	// we need the built frontend as well as the deployed CDK in order to run this step.
	mg.Deps(Build.Frontend, mg.F(Deploy.StagingCDK, env, name))
	ctx := context.Background()
	dep := deploy.NewStagingConfig(ctx, name)

	// upload the frontend to S3 and invalidate CloudFront
	cdkout, err := dep.LoadOutput(ctx)
	if err != nil {
		return err
	}

	return cdkout.DeployFrontend()
}

// StagingDNS sets a DNS CNAME entry in Route53 pointing to the CloudFront domain.
func (Deploy) StagingDNS(env, name string) error {
	mg.Deps(mg.F(Deploy.StagingCDK, env, name))
	ctx := context.Background()
	dep := deploy.NewStagingConfig(ctx, name)
	return dep.SetDNSRecord(ctx)
}

// Staging provisions a staging environment
// env should be 'dev' or 'test' to match a CDK internal deployment environment
func (Deploy) Staging(env, name string) {
	mg.Deps(
		mg.F(Deploy.StagingCDK, env, name),
		mg.F(Deploy.StagingFrontend, env, name),
		mg.F(Deploy.StagingDNS, env, name),
	)
}

func (Deploy) Production(ctx context.Context, releaseBucket, versionHash, stackName, cfnParamsJSON string) error {
	zap.S().Infow("deploying production stack")
	templateURL := fmt.Sprintf("https://%s.s3.amazonaws.com/%s/Granted.template.json", releaseBucket, versionHash)
	exists, err := deploy.StackExists(ctx, stackName)
	if err != nil {
		return err
	}
	if exists {
		// change set
		args := []string{"cloudformation", "update-stack", "--stack-name", stackName, "--capabilities", "CAPABILITY_IAM", "--template-url", templateURL, "--parameters", cfnParamsJSON}
		err = sh.Run("aws", args...)
		if err != nil {
			return err
		}
	} else {
		// create
		args := []string{"cloudformation", "create-stack", "--stack-name", stackName, "--capabilities", "CAPABILITY_IAM", "--template-url", templateURL, "--parameters", cfnParamsJSON}
		err = sh.Run("aws", args...)
		if err != nil {
			return err
		}
	}

	return nil
}

type Release mg.Namespace

func (Release) ProductionCDK(releaseBucket, versionHash string) error {
	mg.Deps(Deps.NPM)
	boldYellow := color.New(color.FgYellow, color.Bold)
	boldYellow.Printf("synthesizing production infrastructure for path %s in bucket %s\n", versionHash, releaseBucket)

	// infrastructure/cdk.json defines a build step within CDK which calls `go run mage.go build`,
	// so we don't need to build or package the code before running cdk deploy.
	// add a '--require-approval never' arg to avoid being prompted for approval in CI pipelines.
	args := []string{"--dir", "deploy/infra", "cdk", "synth", "--require-approval", "never", "--outputs-file", "cdk-outputs.json", "--quiet"}

	dep := deploy.Release{
		ProductionReleasesBucket:      releaseBucket,
		ProductionReleaseBucketPrefix: versionHash,
	}
	args = append(args, dep.CDKContextArgs()...)

	return sh.RunWith(map[string]string{"STACK_TARGET": "prod"}, "pnpm", args...)
}

func (Release) PublishCDKAssets(releaseBucket, versionHash string) error {
	mg.Deps(mg.F(Release.ProductionCDK, releaseBucket, versionHash))
	args := []string{"--dir", "deploy/infra", "publisher"}
	return sh.Run("pnpm", args...)
}

func (Release) PublishCloudFormation(releaseBucket, versionHash string) error {
	mg.Deps(mg.F(Release.ProductionCDK, releaseBucket, versionHash))
	zap.S().Infow("uploading CloudFormation to S3", "bucket", releaseBucket)
	return sh.Run("aws", "s3", "cp", "./deploy/infra/cdk.out/Granted.template.json", fmt.Sprintf("s3://%s/%s/Granted.template.json", releaseBucket, versionHash))
}

func (Release) PublishFrontendAssets(releaseBucket, versionHash string) error {
	mg.Deps(Build.Frontend)
	zap.S().Infow("uploading frontend assets to s3", "bucket", releaseBucket)
	return sh.Run("aws", "s3", "cp", "./web/dist", fmt.Sprintf("s3://%s/%s/frontend-assets/", releaseBucket, versionHash), "--recursive")
}

// PublishManifest updates the manifest.json file in the release bucket with the latest version information,
// so that our customer deployment tooling knows there is a new version available.
func (Release) PublishManifest(releaseBucket, version string) error {
	ctx := context.Background()
	return deploy.PublishManifest(ctx, releaseBucket, version)
}

func (Release) Production(releaseBucket, versionHash string) {
	mg.Deps(
		mg.F(Release.PublishCDKAssets, releaseBucket, versionHash),
		mg.F(Release.PublishFrontendAssets, releaseBucket, versionHash),
		mg.F(Release.PublishCloudFormation, releaseBucket, versionHash),
	)

	// only update the manifest if all of the above steps have succeeded.
	// otherwise, we'll point to a broken deployment.
	mg.Deps(
		mg.F(Release.PublishManifest, releaseBucket, versionHash),
	)
}

// Watch hot-reloads the CDK deployment when local files change.
func Watch() error {
	mg.Deps(Deps.NPM)
	args := []string{"--dir", "deploy/infra", "cdk", "watch", "--outputs-file", "cdk-outputs.json"}

	cfg, err := deploy.LoadConfig(deploy.DefaultFilename)
	if err != nil {
		return err
	}
	args = append(args, cfg.CDKContextArgs()...)

	return sh.Run("pnpm", args...)
}

// Destroy deprovisions the CDK stack.
func Destroy() error {
	mg.Deps(Deps.NPM)
	args := []string{"--dir", "deploy/infra", "cdk", "destroy"}

	cfg, err := deploy.LoadConfig(deploy.DefaultFilename)
	if err != nil {
		return err
	}
	args = append(args, cfg.CDKContextArgs()...)

	return sh.Run("pnpm", args...)
}

// Clean removes build and packaging artifacts.
func Clean() error {
	fmt.Println("cleaning next/out folder...")
	return os.RemoveAll("next/out")
}

// DevConfig sets up the deployment.yml file
func DevConfig() error {
	_, err := deploy.LoadConfig(deploy.DefaultFilename)
	if err != nil && err != deploy.ErrConfigNotExist {
		return err
	}

	if err == nil {
		fmt.Println("deployment.yml already exists: skipping setup")
		return nil
	}
	_, err = deploy.LoadConfig(deploy.DeprecatedDefaultFilename)
	if err != nil && err != deploy.ErrConfigNotExist {
		return err
	}
	if err == nil {
		deploy.DeprecatedDefaultFilenameWarning.Print()
	}

	c, err := deploy.SetupDevConfig()
	if err != nil {
		return err
	}

	err = c.Save(deploy.DefaultFilename)
	if err != nil {
		return err
	}

	fmt.Println("wrote deployment.yml")
	return nil
}

// ensureCDKOutput ensures that the CDK output has been created.
// If it doesn't exist yet it adds a dependency for the CDK deploy
// task, which will create the output, and then tries to read it again.
func ensureCDKOutput() (deploy.Output, error) {
	dc, err := deploy.LoadConfig(deploy.DefaultFilename)
	if err != nil {
		return deploy.Output{}, err
	}
	ctx := context.Background()
	output, err := dc.LoadOutput(ctx)
	if err == deploy.ErrConfigNotExist {
		fmt.Println("CDK output doesn't exist yet, provisioning CDK stack...")
		mg.Deps(Deploy.CDK)

		// try again
		output, err = dc.LoadOutput(ctx)
	}
	return output, err
}
