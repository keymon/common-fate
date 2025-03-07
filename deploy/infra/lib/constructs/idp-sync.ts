import { Duration, Stack } from "aws-cdk-lib";
import { Table } from "aws-cdk-lib/aws-dynamodb";
import * as events from "aws-cdk-lib/aws-events";
import * as targets from "aws-cdk-lib/aws-events-targets";
import { PolicyStatement } from "aws-cdk-lib/aws-iam";
import * as lambda from "aws-cdk-lib/aws-lambda";
import { Construct } from "constructs";
import * as path from "path";
import { grantAssumeIdentitySyncRole } from "../helpers/permissions";
import { WebUserPool } from "./app-user-pool";

interface Props {
  dynamoTable: Table;
  userPool: WebUserPool;
  identityProviderSyncConfiguration: string;
  analyticsDisabled: string;
  analyticsUrl: string;
  analyticsLogLevel: string;
  analyticsDeploymentStage: string;
  identityGroupFilter: string;
  idpSyncTimeoutSeconds: number;
  idpSyncSchedule: string;
  idpSyncMemory: number;
}

export class IdpSync extends Construct {
  private _lambda: lambda.Function;
  private eventRule: events.Rule;

  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id);
    const code = lambda.Code.fromAsset(
      path.join(__dirname, "..", "..", "..", "..", "bin", "syncer.zip")
    );

    this._lambda = new lambda.Function(this, "HandlerFunction", {
      code,
      timeout: Duration.seconds(props.idpSyncTimeoutSeconds),
      memorySize: props.idpSyncMemory,

      environment: {
        COMMONFATE_TABLE_NAME: props.dynamoTable.tableName,
        COMMONFATE_IDENTITY_PROVIDER: props.userPool.getIdpType(),
        COMMONFATE_COGNITO_USER_POOL_ID: props.userPool.getUserPoolId(),
        COMMONFATE_IDENTITY_SETTINGS: props.identityProviderSyncConfiguration,
        CF_ANALYTICS_DISABLED: props.analyticsDisabled,
        CF_ANALYTICS_URL: props.analyticsUrl,
        CF_ANALYTICS_LOG_LEVEL: props.analyticsLogLevel,
        CF_ANALYTICS_DEPLOYMENT_STAGE: props.analyticsDeploymentStage,
        COMMONFATE_IDENTITY_GROUP_FILTER: props.identityGroupFilter,
      },
      runtime: lambda.Runtime.GO_1_X,
      handler: "syncer",
    });

    props.dynamoTable.grantReadWriteData(this._lambda);

    //add event bridge trigger to lambda
    this.eventRule = new events.Rule(this, "EventBridgeCronRule", {
      schedule: events.Schedule.expression(props.idpSyncSchedule),
    });

    // add the Lambda function as a target for the Event Rule
    this.eventRule.addTarget(new targets.LambdaFunction(this._lambda));

    // allow the Event Rule to invoke the Lambda function
    targets.addLambdaPermission(this.eventRule, this._lambda);

    this._lambda.addToRolePolicy(
      new PolicyStatement({
        resources: [props.userPool.getUserPool().userPoolArn],
        actions: [
          "cognito-idp:AdminListGroupsForUser",
          "cognito-idp:ListUsers",
          "cognito-idp:ListGroups",
          "cognito-idp:ListUsersInGroup",
          "cognito-idp:AdminGetUser",
          "cognito-idp:AdminListUserAuthEvents",
          "cognito-idp:AdminUserGlobalSignOut",
          "cognito-idp:DescribeUserPool",
        ],
      })
    );
    this._lambda.addToRolePolicy(
      new PolicyStatement({
        actions: ["ssm:GetParameter"],
        resources: [
          `arn:aws:ssm:${Stack.of(this).region}:${
            Stack.of(this).account
          }:parameter/granted/secrets/identity/*`,
        ],
      })
    );

    grantAssumeIdentitySyncRole(this._lambda);
    //allow the lambda to write to the table
    props.dynamoTable.grantWriteData(this._lambda);
  }
  getLogGroupName(): string {
    return this._lambda.logGroup.logGroupName;
  }
  getFunctionName(): string {
    return this._lambda.functionName;
  }
  getExecutionRoleArn(): string {
    return this._lambda.role?.roleArn || "";
  }
}
