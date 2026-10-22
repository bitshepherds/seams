data "terraform_remote_state" "network" {
  backend = "s3"
  config = {
    bucket = "acme-terraform"
    key    = "network.tfstate"
  }
}
