output "cluster_id" {
  description = "EKS cluster ID."
  value       = module.eks.cluster_id
}

output "cluster_endpoint" {
  description = "Endpoint for EKS control plane."
  value       = module.eks.cluster_endpoint
}

output "cluster_name" {
  value = local.cluster_name
}

output "irsa_role_arn" {
  value = module.irsa_ack_role.iam_role_arn
}

output "aws_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "oidc_provider" {
  value = local.oidc_provider_stripped
}
