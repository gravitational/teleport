# Teleport Database

resource "teleport_database" "example" {
    metadata = {
        name = "example"
        description = "Test database"
        labels = {
            "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
        }
    }

    spec = {
        protocol = "postgres"
        uri = "localhost"
    }
}