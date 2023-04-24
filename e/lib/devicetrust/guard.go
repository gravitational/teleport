package devicetrust

// MDMFeatureActive is a temporary guard for Device Trust MDM integration
// features, currently planned for Teleport 13.1.
// TODO(codingllama): Use a single guard from the OSS package.
var MDMFeatureActive = false

// AutoEnrollEnabled guards the automatic enrollment feature.
// TODO(codingllama): Use a proper config toggle.
var AutoEnrollEnabled = false
