output "client_id" {
  value = azuread_application.app.client_id
}

output "tenant_id" {
  value = var.tenant_id
}