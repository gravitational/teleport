provider "aws" {
  region  = "us-east-1"
  profile = "loadsmart-main"
}

provider "aws" {
  alias   = "dev"
  region  = "us-east-1"
  profile = "loadsmart-dev"
}
