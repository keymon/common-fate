import * as iam from "aws-cdk-lib/aws-iam";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as sfn from "aws-cdk-lib/aws-stepfunctions";
import { Construct } from "constructs";
import { Duration, Stack } from "aws-cdk-lib";
import * as path from "path";
import { EventBus } from "aws-cdk-lib/aws-events";
import { grantAssumeHandlerRole } from "../helpers/permissions";
import { Table } from "aws-cdk-lib/aws-dynamodb";
import * as ec2 from 'aws-cdk-lib/aws-ec2'

interface Props {
  eventBusSourceName: string;
  dynamoTable: Table;
  eventBus: EventBus;
  lambdaVpcId: string,
}
export class TargetGroupGranter extends Construct {
  private _stateMachine: sfn.StateMachine;
  private _lambda: lambda.Function;
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id);
    const code = lambda.Code.fromAsset(
      path.join(
        __dirname,
        "..",
        "..",
        "..",
        "..",
        "bin",
        "targetgroup-granter.zip"
      )
    );

    const vpc = ec2.Vpc.fromLookup(this, 'Vpc', {
      vpcId: props.lambdaVpcId,
    });

    this._lambda = new lambda.Function(this, "StepHandlerFunction", {
      code,
      timeout: Duration.minutes(5),
      environment: {
        COMMONFATE_EVENT_BUS_ARN: props.eventBus.eventBusArn,
        COMMONFATE_EVENT_BUS_SOURCE: props.eventBusSourceName,
        COMMONFATE_TABLE_NAME: props.dynamoTable.tableName,
      },
      runtime: lambda.Runtime.GO_1_X,
      handler: "targetgroup-granter",
      vpc: vpc,
    });

    props.dynamoTable.grantReadWriteData(this._lambda);
    props.eventBus.grantPutEventsTo(this._lambda);

    grantAssumeHandlerRole(this._lambda);

    // this lambda needs to be able to invoke provider deployments
    const definition = {
      StartAt: "Validate End is in the Future",
      States: {
        "Validate End is in the Future": {
          Type: "Choice",
          Choices: [
            {
              Variable: "$.grant.end",
              TimestampGreaterThanPath: "$$.State.EnteredTime",
              Next: "Wait for Grant Start Time",
            },
          ],
          Default: "Fail",
          Comment: "Do not provision any access if the end time is in the past",
        },
        "Wait for Grant Start Time": {
          Type: "Wait",
          TimestampPath: "$.grant.start",
          Next: "Activate Access",
        },
        "Activate Access": {
          Type: "Task",
          Resource: "arn:aws:states:::lambda:invoke",
          Parameters: {
            FunctionName: this._lambda.functionArn,
            Payload: {
              "action": "ACTIVATE",
              "grant.$": "$.grant",
            },
          },
          Retry: [
            {
              ErrorEquals: [
                "Lambda.ServiceException",
                "Lambda.AWSLambdaException",
                "Lambda.SdkClientException",
              ],
              IntervalSeconds: 2,
              MaxAttempts: 6,
              BackoffRate: 2,
            },
          ],
          Next: "Wait for Window End",
          ResultPath: "$",
          OutputPath: "$.Payload",
        },
        "Wait for Window End": {
          Type: "Wait",
          TimestampPath: "$.grant.end",
          Next: "Expire Access",
        },
        "Expire Access": {
          Type: "Task",
          Resource: "arn:aws:states:::lambda:invoke",
          OutputPath: "$.Payload",
          Parameters: {
            FunctionName: this._lambda.functionArn,
            // This passes the output into the revoke action which may or may not include state
            Payload: {
              "action": "DEACTIVATE",
              "grant.$": "$.grant",
              "state.$": "$.state",
            },
          },
          Retry: [
            {
              ErrorEquals: [
                "Lambda.ServiceException",
                "Lambda.AWSLambdaException",
                "Lambda.SdkClientException",
              ],
              IntervalSeconds: 2,
              MaxAttempts: 6,
              BackoffRate: 2,
            },
          ],
          ResultPath: "$",
          End: true,
        },
        "Fail": {
          Type: "Fail",
        },
      },
      Comment: "Common Fate Access Workflow State Machine",
    };

    this._stateMachine = new sfn.StateMachine(this, "StateMachine", {
      definition: new sfn.Pass(this, "StartState"),
    });

    const cfnStatemachine = this._stateMachine.node
      .defaultChild as sfn.CfnStateMachine;

    cfnStatemachine.definitionString = JSON.stringify(definition);

    const smRole = iam.Role.fromRoleArn(
      this,
      "StateMachineRole",
      cfnStatemachine.roleArn
    );
    this._lambda.grantInvoke(smRole);
  }
  getStateMachineARN(): string {
    return this._stateMachine.stateMachineArn;
  }
  getStateMachine(): sfn.StateMachine {
    return this._stateMachine;
  }
  getLogGroupName(): string {
    return this._lambda.logGroup.logGroupName;
  }
  getGranterARN(): string {
    return this._lambda.functionArn;
  }
}
