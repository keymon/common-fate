import * as cdk from "aws-cdk-lib";

import { Construct } from "constructs";
import { AccessHandler } from "./constructs/access-handler";
import { AppBackend } from "./constructs/app-backend";
import { AppFrontend } from "./constructs/app-frontend";
import { WebUserPool } from "./constructs/app-user-pool";
import * as kms from "aws-cdk-lib/aws-kms";

import { CfnParameter } from "aws-cdk-lib";
import { EventBus } from "./constructs/events";
import { ProductionFrontendDeployer } from "./constructs/production-frontend-deployer";
import { generateOutputs } from "./helpers/outputs";
import {
  IdentityProviderRegistry,
  IdentityProviderTypes,
} from "./helpers/registry";
import { Database } from "./constructs/database";
import { Governance } from "./constructs/governance";
import { TargetGroupGranter } from "./constructs/targetgroup-granter";

interface Props extends cdk.StackProps {
  productionReleasesBucket: string;
  productionFrontendAssetObjectPrefix: string;
}
export class CommonFateStackProd extends cdk.Stack {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, props);

    const cognitoDomainPrefix = new CfnParameter(this, "CognitoDomainPrefix", {
      type: "String",
      description:
        "CognitoDomainPrefix is a globally unique cognito domain prefix.",
      minLength: 1,
    });

    const idpType = new CfnParameter(this, "IdentityProviderType", {
      type: "String",
      description:
        "Configure your identity provider, okta requires SamlSSOMetadataURL to be provided",
      default: IdentityProviderRegistry.Cognito,
      allowedValues: Object.values(IdentityProviderRegistry),
    });

    const samlMetadataUrl = new CfnParameter(this, "SamlSSOMetadataURL", {
      type: "String",
      description:
        "Add your metadata url here to enable SSO, optionally leave this empty and provide the full metadata xml via SamlSSOMetadata",
      default: "",
    });
    const samlMetadata = new CfnParameter(this, "SamlSSOMetadata", {
      type: "String",
      description:
        "Add your metadata here to enable SSO, optionally, leave this empty and provide a metadata url SamlSSOMetadataURL",
      default: "",
    });

    const administratorGroupId = new CfnParameter(
      this,
      "AdministratorGroupID",
      {
        type: "String",
        description:
          "Required, if you are not using cognito for your users you will need to provide a group id from your IDP which will control who has access to the administrator functions.",
        default: "common_fate_administrators",
      }
    );

    const suffix = new CfnParameter(this, "DeploymentSuffix", {
      type: "String",
      description:
        "An optional suffix to be added to deployed resources (useful for testing scenarios where multiple stacks are deployed to a single AWS account)",
      default: "",
    });

    const frontendDomain = new CfnParameter(this, "FrontendDomain", {
      type: "String",
      description:
        "An optional custom domain name for the Common Fate web application. If not provided, an auto-generated CloudFront URL will be used.",
      default: "",
    });

    const frontendCertificate = new CfnParameter(
      this,
      "FrontendCertificateARN",
      {
        type: "String",
        description:
          "The ARN of an ACM certificate in us-east-1 for the frontend URL. Must be set if 'FrontendDomain' is defined.",
        default: "",
      }
    );

    const providerConfig = new CfnParameter(this, "ProviderConfiguration", {
      type: "String",
      description: "The Access Provider configuration in JSON format",
      default: "{}",
    });
    const notificationsConfiguration = new CfnParameter(
      this,
      "NotificationsConfiguration",
      {
        type: "String",
        description: "The Notifications configuration in JSON format",
        default: "{}",
      }
    );
    const identityConfig = new CfnParameter(this, "IdentityConfiguration", {
      type: "String",
      description: "The Identity Provider Sync configuration in JSON format",
      default: "{}",
    });

    const remoteConfigUrl = new CfnParameter(
      this,
      "ExperimentalRemoteConfigURL",
      {
        type: "String",
        description:
          "If provided, sources configuration from an API, rather than deployment parameters.",
        default: "",
      }
    );

    const identityGroupFilter = new CfnParameter(this, "IdentityGroupFilter", {
      type: "String",
      description:
        "If provided, only groups matching this filter will be synced to the Common Fate database.",
      default: "",
    });

    const remoteConfigHeaders = new CfnParameter(
      this,
      "ExperimentalRemoteConfigHeaders",
      {
        type: "String",
        description: "Headers to include in the remote config API calls",
        default: "",
      }
    );

    //     IDPSyncTimeoutSeconds
    // IDPSyncSchedule
    // IDPSyncMemory

    const idpSyncTimeoutSeconds = new CfnParameter(
      this,
      "IDPSyncTimeoutSeconds",
      {
        type: "Number",
        description: "Timeout for IDP Sync Lambda Function",
        default: 30,
      }
    );

    const idpSyncSchedule = new CfnParameter(this, "IDPSyncSchedule", {
      type: "String",
      description: "Cron schedule for IDP Sync Lambda Function",
      default: "rate(5 minutes)",
    });

    const idpSyncMemory = new CfnParameter(this, "IDPSyncMemory", {
      type: "Number",
      description: "Memory for IDP Sync Lambda Function",
      default: 128,
    });

    const cloudfrontWafAclArn = new CfnParameter(this, "CloudfrontWAFACLARN", {
      type: "String",
      description:
        "The ARN of a WAF ACL instance which is configured for the Cloudfront distribution.",
      default: "",
    });
    const apiGatewayWafAclArn = new CfnParameter(this, "APIGatewayWAFACLARN", {
      type: "String",
      description:
        "The ARN of a WAF ACL instance which is configured for the API Gateway.",
      default: "",
    });

    const analyticsUrl = new CfnParameter(this, "AnalyticsURL", {
      type: "String",
      description: "A custom URL to send anonymous analytics to.",
      default: "",
    });

    const analyticsDisabled = new CfnParameter(this, "AnalyticsDisabled", {
      type: "String",
      description: "Disable anonymous analytics",
      default: "false",
      allowedValues: ["", "true", "false"],
    });

    const analyticsLogLevel = new CfnParameter(this, "AnalyticsLogLevel", {
      type: "String",
      description: "Analytics logging level",
      default: "",
    });

    const analyticsDeploymentStage = new CfnParameter(
      this,
      "AnalyticsDeploymentStage",
      {
        type: "String",
        description: "A label for the deployment stage (dev, uat)",
        default: "",
      }
    );

    const lambdaVpcId = new CfnParameter(
        this,
        "LambdaVpcId",
        {
          type: "String",
          description: "VPC that grant lambda will be attached to.",
          default: "",
        }
    )

    const appName = this.stackName + suffix.valueAsString;

    const db = new Database(this, "Database", {
      appName,
    });

    const appFrontend = new AppFrontend(this, "Frontend", {
      appName,
      // this is the same for all prod synthesis, it means that you can only deploy this once per account in production mode event with the suffix.
      // because the suffix cannot be appended to a logical id as it is a token.
      // the logical id must remain static to avoid issues with updates
      stableName: this.stackName,
    }).withProdCDN({
      frontendDomain: frontendDomain.valueAsString,
      frontendCertificateArn: frontendCertificate.valueAsString,
      cloudfrontWafAclArn: cloudfrontWafAclArn.valueAsString,
    });

    const userPool = new WebUserPool(this, "WebUserPool", {
      appName,
      domainPrefix: cognitoDomainPrefix.valueAsString,
      callbackUrls: appFrontend.getProdCallbackUrls(),
      idpType: idpType.valueAsString as IdentityProviderTypes,
      samlMetadataUrl: samlMetadataUrl.valueAsString,
      samlMetadata: samlMetadata.valueAsString,
      devConfig: null,
      frontendUrl: "https://" + appFrontend.getDomainName(),
    });

    const events = new EventBus(this, "EventBus", {
      appName,
    });

    const accessHandler = new AccessHandler(this, "AccessHandler", {
      appName,
      eventBus: events.getEventBus(),
      eventBusSourceName: events.getEventBusSourceName(),
      providerConfig: providerConfig.valueAsString,
      remoteConfigUrl: remoteConfigUrl.valueAsString,
      remoteConfigHeaders: remoteConfigHeaders.valueAsString,
    });

    //KMS key is used in governance api as well as appBackend - both for tokinization for ddb use
    const kmsKey = new kms.Key(this, "PaginationKMSKey", {
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      pendingWindow: cdk.Duration.days(7),
      enableKeyRotation: true,
      description:
        "Used for encrypting and decrypting pagination tokens for Common Fate",
    });

    const governance = new Governance(this, "Governance", {
      appName,
      accessHandler: accessHandler,
      kmsKey: kmsKey,
      providerConfig: providerConfig.valueAsString,
      dynamoTable: db.getTable(),
    });
    const targetGroupGranter = new TargetGroupGranter(
      this,
      "TargetGroupGranter",
      {
        eventBus: events.getEventBus(),
        eventBusSourceName: events.getEventBusSourceName(),
        dynamoTable: db.getTable(),
        lambdaVpcId: lambdaVpcId.valueAsString
      }
    );
    const appBackend = new AppBackend(this, "API", {
      appName,
      userPool: userPool,
      frontendUrl: "https://" + appFrontend.getDomainName(),
      accessHandler: accessHandler,
      governanceHandler: governance,
      eventBus: events.getEventBus(),
      eventBusSourceName: events.getEventBusSourceName(),
      adminGroupId: administratorGroupId.valueAsString,
      identityProviderSyncConfiguration: identityConfig.valueAsString,
      notificationsConfiguration: notificationsConfiguration.valueAsString,
      providerConfig: providerConfig.valueAsString,
      deploymentSuffix: suffix.valueAsString,
      dynamoTable: db.getTable(),
      remoteConfigUrl: remoteConfigUrl.valueAsString,
      remoteConfigHeaders: remoteConfigHeaders.valueAsString,
      apiGatewayWafAclArn: apiGatewayWafAclArn.valueAsString,
      analyticsDisabled: analyticsDisabled.valueAsString,
      analyticsUrl: analyticsUrl.valueAsString,
      analyticsLogLevel: analyticsLogLevel.valueAsString,
      analyticsDeploymentStage: analyticsDeploymentStage.valueAsString,
      kmsKey: kmsKey,
      shouldRunCronHealthCheckCacheSync: true,
      idpSyncMemory: idpSyncMemory.valueAsNumber,
      idpSyncSchedule: idpSyncSchedule.valueAsString,
      idpSyncTimeoutSeconds: idpSyncTimeoutSeconds.valueAsNumber,
      targetGroupGranter: targetGroupGranter,
      identityGroupFilter: identityGroupFilter.valueAsString,
    });

    new ProductionFrontendDeployer(this, "FrontendDeployer", {
      apiUrl: appBackend.getRestApiURL(),
      cloudfrontDistributionId: appFrontend.getDistributionId(),
      frontendDomain: appFrontend.getDomainName(),
      frontendBucket: appFrontend.getBucket(),
      cognitoClientId: userPool.getUserPoolClientId(),
      cognitoUserPoolId: userPool.getUserPoolId(),
      userPoolDomain: userPool.getUserPoolLoginFQDN(),
      cfReleaseBucket: props.productionReleasesBucket,
      cfReleaseBucketFrontendAssetObjectPrefix:
        props.productionFrontendAssetObjectPrefix,
      cliClientId: userPool.getCLIAppClient().userPoolClientId,
    });

    /* Outputs */
    generateOutputs(this, {
      CognitoClientID: userPool.getUserPoolClientId(),
      CloudFrontDomain: appFrontend.getCloudFrontDomain(),
      FrontendDomainOutput: appFrontend.getDomainName(),
      CloudFrontDistributionID: appFrontend.getDistributionId(),
      S3BucketName: appFrontend.getBucketName(),
      UserPoolID: userPool.getUserPoolId(),
      UserPoolDomain: userPool.getUserPoolLoginFQDN(),
      APIURL: appBackend.getRestApiURL(),
      WebhookURL: appBackend.getWebhookApiURL(),
      GovernanceURL: governance.getGovernanceApiURL(),
      APILogGroupName: appBackend.getLogGroupName(),
      WebhookLogGroupName: appBackend.getWebhookLogGroupName(),
      IDPSyncLogGroupName: appBackend.getIdpSync().getLogGroupName(),
      AccessHandlerLogGroupName: accessHandler.getLogGroupName(),
      EventBusLogGroupName: events.getLogGroupName(),
      EventsHandlerLogGroupName: appBackend.getEventHandler().getLogGroupName(),
      GranterLogGroupName: accessHandler.getGranter().getLogGroupName(),
      SlackNotifierLogGroupName: appBackend
        .getNotifiers()
        .getSlackLogGroupName(),
      GovernanceAPILogGroupName: governance.getGovernanceLogGroupName(),
      DynamoDBTable: appBackend.getDynamoTableName(),
      GranterStateMachineArn: accessHandler.getGranter().getStateMachineARN(),
      EventBusArn: events.getEventBus().eventBusArn,
      EventBusSource: events.getEventBusSourceName(),
      IdpSyncFunctionName: appBackend.getIdpSync().getFunctionName(),
      SAMLIdentityProviderName:
        userPool.getSamlUserPoolClient()?.getUserPoolName() || "",
      Region: this.region,
      PaginationKMSKeyARN: appBackend.getKmsKeyArn(),
      AccessHandlerExecutionRoleARN:
        accessHandler.getAccessHandlerExecutionRoleArn(),
      CacheSyncLogGroupName: appBackend.getCacheSync().getLogGroupName(),
      IDPSyncExecutionRoleARN: appBackend.getIdpSync().getExecutionRoleArn(),
      RestAPIExecutionRoleARN: appBackend.getExecutionRoleArn(),
      CacheSyncFunctionName: appBackend.getCacheSync().getFunctionName(),
      CLIAppClientID: userPool.getCLIAppClient().userPoolClientId,
      HealthcheckFunctionName: appBackend.getHealthChecker().getFunctionName(),
      HealthcheckLogGroupName: appBackend.getHealthChecker().getLogGroupName(),
      GranterV2StateMachineArn: targetGroupGranter.getStateMachineARN(),
    });
  }
}
