package config

var defaultStackTemplate = `
{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Description": "kube-aws Kubernetes cluster {{.ClusterName}}",
  "Parameters" : {
    {{with .ExistingVPC}}
      "VPC" : {
        "Type" : "String",
        "Default" : "{{.VPCID}}",
        "Description" : "ID of kubernetes VPC"
      },
      "RouteTable": {
        "Type" : "String",
        "Default" : "{{.RouteTableID}}",
	    "Description" : "Route Table to attach to"
      }
    {{end}}
  },
  "Resources": {
    "AlarmControllerRecover": {
      "Properties": {
        "AlarmActions": [
          {
            "Fn::Join": [
              "",
              [
                "arn:aws:automate:",
                {
                  "Ref": "AWS::Region"
                },
                ":ec2:recover"
              ]
            ]
          }
        ],
        "AlarmDescription": "Trigger a recovery when system check fails for 5 consecutive minutes.",
        "ComparisonOperator": "GreaterThanThreshold",
        "Dimensions": [
          {
            "Name": "InstanceId",
            "Value": {
              "Ref": "InstanceController"
            }
          }
        ],
        "EvaluationPeriods": "5",
        "MetricName": "StatusCheckFailed_System",
        "Namespace": "AWS/EC2",
        "Period": "60",
        "Statistic": "Minimum",
        "Threshold": "0"
      },
      "Type": "AWS::CloudWatch::Alarm"
    },
    "AutoScaleWorker": {
      "Properties": {
        "AvailabilityZones": [
          "{{.AvailabilityZone}}"
        ],
        "DesiredCapacity": "{{.WorkerCount}}",
        "HealthCheckGracePeriod": 600,
        "HealthCheckType": "EC2",
        "LaunchConfigurationName": {
          "Ref": "LaunchConfigurationWorker"
        },
        "MaxSize": "{{.WorkerCount}}",
        "MinSize": "{{.WorkerCount}}",
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "PropagateAtLaunch": "true",
            "Value": "{{.ClusterName}}"
          },
          {
            "Key": "Name",
            "PropagateAtLaunch": "true",
            "Value": "kube-aws-worker"
          }
        ],
        "VPCZoneIdentifier": [
          {
            "Ref": "Subnet"
          }
        ]
      },
      "Type": "AWS::AutoScaling::AutoScalingGroup",
      "UpdatePolicy" : {
	    "AutoScalingRollingUpdate" : {
          "MinInstancesInService" :
          {{if .WorkerSpotPrice}}
            "0"
          {{else}}
            "{{.WorkerCount}}"
          {{end}},
          "MaxBatchSize" : "1",
          "PauseTime" : "PT2M"
	    }
      }
    },
    "EIPController": {
      "Properties": {
        "Domain": "vpc",
        "InstanceId": {
          "Ref": "InstanceController"
        }
      },
      "Type": "AWS::EC2::EIP"
    },
    "IAMInstanceProfileController": {
      "Properties": {
        "Path": "/",
        "Roles": [
          {
            "Ref": "IAMRoleController"
          }
        ]
      },
      "Type": "AWS::IAM::InstanceProfile"
    },
    "IAMInstanceProfileWorker": {
      "Properties": {
        "Path": "/",
        "Roles": [
          {
            "Ref": "IAMRoleWorker"
          }
        ]
      },
      "Type": "AWS::IAM::InstanceProfile"
    },
    "IAMRoleController": {
      "Properties": {
        "AssumeRolePolicyDocument": {
          "Statement": [
            {
              "Action": [
                "sts:AssumeRole"
              ],
              "Effect": "Allow",
              "Principal": {
                "Service": [
                  "ec2.amazonaws.com"
                ]
              }
            }
          ],
          "Version": "2012-10-17"
        },
        "Path": "/",
        "Policies": [
          {
            "PolicyDocument": {
              "Statement": [
                {
                  "Action": "ec2:*",
                  "Effect": "Allow",
                  "Resource": "*"
                },
                {
                  "Action": "elasticloadbalancing:*",
                  "Effect": "Allow",
                  "Resource": "*"
                }
              ],
              "Version": "2012-10-17"
            },
            "PolicyName": "root"
          }
        ]
      },
      "Type": "AWS::IAM::Role"
    },
    "IAMRoleWorker": {
      "Properties": {
        "AssumeRolePolicyDocument": {
          "Statement": [
            {
              "Action": [
                "sts:AssumeRole"
              ],
              "Effect": "Allow",
              "Principal": {
                "Service": [
                  "ec2.amazonaws.com"
                ]
              }
            }
          ],
          "Version": "2012-10-17"
        },
        "Path": "/",
        "Policies": [
          {
            "PolicyDocument": {
              "Statement": [
                {
                  "Action": "ec2:Describe*",
                  "Effect": "Allow",
                  "Resource": "*"
                },
                {
                  "Action": "ec2:AttachVolume",
                  "Effect": "Allow",
                  "Resource": "*"
                },
                {
                  "Action": "ec2:DetachVolume",
                  "Effect": "Allow",
                  "Resource": "*"
                }
              ],
              "Version": "2012-10-17"
            },
            "PolicyName": "root"
          }
        ]
      },
      "Type": "AWS::IAM::Role"
    },
    "InstanceController": {
      "Properties": {
        "AvailabilityZone": "{{.AvailabilityZone}}",
        "BlockDeviceMappings": [
          {
            "DeviceName": "/dev/xvda",
            "Ebs": {
              "VolumeSize": "{{.ControllerRootVolumeSize}}"
            }
          }
        ],
        "IamInstanceProfile": {
          "Ref": "IAMInstanceProfileController"
        },
        "ImageId": "{{.AMI}}",
        "InstanceType": "{{.ControllerInstanceType}}",
        "KeyName": "{{.KeyName}}",
        "NetworkInterfaces": [
          {
            "AssociatePublicIpAddress": false,
            "DeleteOnTermination": true,
            "DeviceIndex": "0",
            "GroupSet": [
              {
                "Ref": "SecurityGroupController"
              }
            ],
            "PrivateIpAddress": "{{.ControllerIP}}",
            "SubnetId": {
              "Ref": "Subnet"
            }
          }
        ],
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          },
          {
            "Key": "Name",
            "Value": "kube-aws-controller"
          }
        ],
        "UserData": "{{.UserData.Controller.String}}"
      },
      "Type": "AWS::EC2::Instance"
    },
    "LaunchConfigurationWorker": {
      "Properties": {
        "BlockDeviceMappings": [
          {
            "DeviceName": "/dev/xvda",
            "Ebs": {
              "VolumeSize": "{{.WorkerRootVolumeSize}}"
            }
          }
        ],
        "IamInstanceProfile": {
          "Ref": "IAMInstanceProfileWorker"
        },
        "ImageId": "{{.AMI}}",
        "InstanceType": "{{.WorkerInstanceType}}",
        "KeyName": "{{.KeyName}}",
        "SecurityGroups": [
          {
            "Ref": "SecurityGroupWorker"
          }
        ],
        {{if .WorkerSpotPrice}}
        "SpotPrice": {{.WorkerSpotPrice}},
        {{end}}
        "UserData": "{{.UserData.Worker.String}}"
      },
      "Type": "AWS::AutoScaling::LaunchConfiguration"
    },
    "SecurityGroupController": {
      "Properties": {
        "GroupDescription": {
          "Ref": "AWS::StackName"
        },
        "SecurityGroupEgress": [
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 0,
            "IpProtocol": "tcp",
            "ToPort": 65535
          },
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 0,
            "IpProtocol": "udp",
            "ToPort": 65535
          }
        ],
        "SecurityGroupIngress": [
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 3,
            "IpProtocol": "icmp",
            "ToPort": -1
          },
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 22,
            "IpProtocol": "tcp",
            "ToPort": 22
          },
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 443,
            "IpProtocol": "tcp",
            "ToPort": 443
          }
        ],
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          }
        ],
        "VpcId": {
          "Ref": "VPC"
        }
      },
      "Type": "AWS::EC2::SecurityGroup"
    },
    "SecurityGroupControllerIngressFromWorkerToEtcd": {
      "Properties": {
        "FromPort": 2379,
        "GroupId": {
          "Ref": "SecurityGroupController"
        },
        "IpProtocol": "tcp",
        "SourceSecurityGroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "ToPort": 2379
      },
      "Type": "AWS::EC2::SecurityGroupIngress"
    },
    "SecurityGroupWorker": {
      "Properties": {
        "GroupDescription": {
          "Ref": "AWS::StackName"
        },
        "SecurityGroupEgress": [
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 0,
            "IpProtocol": "tcp",
            "ToPort": 65535
          },
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 0,
            "IpProtocol": "udp",
            "ToPort": 65535
          }
        ],
        "SecurityGroupIngress": [
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 3,
            "IpProtocol": "icmp",
            "ToPort": -1
          },
          {
            "CidrIp": "0.0.0.0/0",
            "FromPort": 22,
            "IpProtocol": "tcp",
            "ToPort": 22
          }
        ],
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          }
        ],
        "VpcId": {
          "Ref": "VPC"
        }
      },
      "Type": "AWS::EC2::SecurityGroup"
    },
    "SecurityGroupWorkerIngressFromControllerToFlannel": {
      "Properties": {
        "FromPort": 8285,
        "GroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "IpProtocol": "udp",
        "SourceSecurityGroupId": {
          "Ref": "SecurityGroupController"
        },
        "ToPort": 8285
      },
      "Type": "AWS::EC2::SecurityGroupIngress"
    },
    "SecurityGroupWorkerIngressFromControllerToKubelet": {
      "Properties": {
        "FromPort": 10250,
        "GroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "IpProtocol": "tcp",
        "SourceSecurityGroupId": {
          "Ref": "SecurityGroupController"
        },
        "ToPort": 10250
      },
      "Type": "AWS::EC2::SecurityGroupIngress"
    },
    "SecurityGroupWorkerIngressFromControllerTocAdvisor": {
      "Properties": {
        "FromPort": 4194,
        "GroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "IpProtocol": "tcp",
        "SourceSecurityGroupId": {
          "Ref": "SecurityGroupController"
        },
        "ToPort": 4194
      },
      "Type": "AWS::EC2::SecurityGroupIngress"
    },
    "SecurityGroupWorkerIngressFromWorkerToFlannel": {
      "Properties": {
        "FromPort": 8285,
        "GroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "IpProtocol": "udp",
        "SourceSecurityGroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "ToPort": 8285
      },
      "Type": "AWS::EC2::SecurityGroupIngress"
    },
    "SecurityGroupWorkerIngressFromWorkerToKubeletReadOnly": {
      "Properties": {
        "FromPort": 10255,
        "GroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "IpProtocol": "tcp",
        "SourceSecurityGroupId": {
          "Ref": "SecurityGroupWorker"
        },
        "ToPort": 10255
      },
      "Type": "AWS::EC2::SecurityGroupIngress"
    },
    "Subnet": {
      "Properties": {
        "AvailabilityZone": "{{.AvailabilityZone}}",
        "CidrBlock": "{{.InstanceCIDR}}",
        "MapPublicIpOnLaunch": true,
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          }
        ],
        "VpcId": {
          "Ref": "VPC"
        }
      },
      "Type": "AWS::EC2::Subnet"
    },
    "SubnetRouteTableAssociation": {
      "Properties": {
        "RouteTableId": {
          "Ref": "RouteTable"
        },
        "SubnetId": {
          "Ref": "Subnet"
        }
      },
      "Type": "AWS::EC2::SubnetRouteTableAssociation"
    }{{if not .ExistingVPC }},
    "VPC": {
      "Properties": {
        "CidrBlock": "{{.VPCCIDR}}",
        "EnableDnsHostnames": true,
        "EnableDnsSupport": true,
        "InstanceTenancy": "default",
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          },
          {
            "Key": "Name",
            "Value": "kubernetes-{{.ClusterName}}-vpc"
          }
        ]
      },
      "Type": "AWS::EC2::VPC"
    },
    "VPCGatewayAttachment": {
      "Properties": {
        "InternetGatewayId": {
          "Ref": "InternetGateway"
        },
        "VpcId": {
          "Ref": "VPC"
        }
      },
      "Type": "AWS::EC2::VPCGatewayAttachment"
    },
    "InternetGateway": {
      "Properties": {
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          }
        ]
      },
      "Type": "AWS::EC2::InternetGateway"
    },
    "RouteToInternet": {
      "Properties": {
        "DestinationCidrBlock": "0.0.0.0/0",
        "GatewayId": {
          "Ref": "InternetGateway"
        },
        "RouteTableId": {
          "Ref": "RouteTable"
        }
      },
      "Type": "AWS::EC2::Route"
    },
    "RouteTable": {
      "Properties": {
        "Tags": [
          {
            "Key": "KubernetesCluster",
            "Value": "{{.ClusterName}}"
          }
        ],
        "VpcId": {
          "Ref": "VPC"
        }
      },
      "Type": "AWS::EC2::RouteTable"
    }
    {{end}}
  }
}
`
