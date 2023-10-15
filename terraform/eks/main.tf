provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = var.default_tags
  }
}

data "aws_availability_zones" "available" {
  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

locals {
  cluster_name = "ack-demo"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "ack-demo-vpc"

  cidr = "10.42.0.0/16"
  azs  = slice(data.aws_availability_zones.available.names, 0, 3)

  private_subnets = ["10.42.1.0/24", "10.42.2.0/24", "10.42.3.0/24"]
  public_subnets  = ["10.42.4.0/24", "10.42.5.0/24", "10.42.6.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true

  public_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    "kubernetes.io/role/elb"                      = 1
  }

  private_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    "kubernetes.io/role/internal-elb"             = 1
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "19.15.3"

  cluster_name    = local.cluster_name
  cluster_version = "1.27"

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = true

  eks_managed_node_group_defaults = {
    ami_type = "AL2_x86_64"
  }

  eks_managed_node_groups = {
    one = {
      name = "node-group-1"

      instance_types = ["t3.small"]

      min_size     = 1
      max_size     = 3
      desired_size = 2
      tags = var.default_tags
    }

    two = {
      name = "node-group-2"

      instance_types = ["t3.small"]

      min_size     = 1
      max_size     = 2
      desired_size = 1
      tags = var.default_tags
    }
  }
}
data "aws_caller_identity" "current" {}

locals {
  oidc_provider_stripped = replace(module.eks.oidc_provider, "https://", "")
}

data "aws_iam_policy" "iam_full" {
  arn = "arn:aws:iam::aws:policy/IAMFullAccess"
}

data "aws_iam_policy" "s3_full" {
  arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
}

data "aws_iam_policy" "sns_full" {
  arn = "arn:aws:iam::aws:policy/AmazonSNSFullAccess"
}

data "aws_iam_policy" "sqs_full" {
  arn = "arn:aws:iam::aws:policy/AmazonSQSFullAccess"
}

data "aws_iam_policy" "rds_full" {
  arn = "arn:aws:iam::aws:policy/AmazonRDSFullAccess"
}

module "irsa_ack_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role-with-oidc"
  version = "4.7.0"

  create_role  = true
  role_name    = "ACK-Trust-${module.eks.cluster_name}"
  provider_url = module.eks.oidc_provider

  role_policy_arns = [
    data.aws_iam_policy.iam_full.arn,
    data.aws_iam_policy.s3_full.arn,
    data.aws_iam_policy.sns_full.arn,
    data.aws_iam_policy.sqs_full.arn,
    data.aws_iam_policy.rds_full.arn
  ]

  oidc_fully_qualified_subjects = [
    "system:serviceaccount:ack-system:ack-iam-controller",
    "system:serviceaccount:ack-system:ack-s3-controller",
    "system:serviceaccount:ack-system:ack-sns-controller",
    "system:serviceaccount:ack-system:ack-sqs-controller",
    "system:serviceaccount:ack-system:ack-rds-controller"
  ]
}

