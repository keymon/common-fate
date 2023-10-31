import * as cdk from '@aws-cdk/core';
import * as lambda from '@aws-cdk/aws-lambda';
import * as ec2 from '@aws-cdk/aws-ec2';
import { Construct } from "constructs";

export interface BaseLambdaProps {
  lambdaNetworkConfig?: LambdaNetworkConfig;
}

export interface LambdaNetworkConfig {
  vpcId: string;
  subnetIds: string[];
  securityGroupIds?: string[];
}

export class BaseLambdaConstruct extends Construct {
  protected readonly vpc?: ec2.IVpc;
  protected readonly subnets?: ec2.SubnetSelection;
  protected readonly securityGroups?: ec2.ISecurityGroup[];

  constructor(scope: Construct, id: string, props: BaseLambdaProps) {
    super(scope, id);

    // Use a specific VPC if provided
    this.vpc = props.lambdaNetworkConfig?.vpcId ? ec2.Vpc.fromLookup(this, 'VPC', { vpcId: props.lambdaNetworkConfig.vpcId }) : undefined;

    // Use specific subnets if provided
    this.subnets = props.lambdaNetworkConfig?.subnetIds ? { subnetIds: props.lambdaNetworkConfig.subnetIds } : undefined;

    // Use a specific security group if provided
    if (props.lambdaNetworkConfig?.securityGroupIds) {
  		this.securityGroups = props.lambdaNetworkConfig.securityGroupIds.map(sgId => ec2.SecurityGroup.fromSecurityGroupId(this, `SG-${sgId}`, sgId));
    } else {
  		this.securityGroups = undefined;
    }
  }
}
