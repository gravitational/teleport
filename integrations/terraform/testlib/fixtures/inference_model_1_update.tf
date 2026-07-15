resource "teleport_inference_model" "bedrock" {
  version = "v1"
  metadata = {
    name = "bedrock-model"
  }
  spec = {
    bedrock = {
      region           = "us-east-1"
      bedrock_model_id = "us.amazon.nova-lite-v1:0"
    }
    max_session_length_bytes = 200000
  }
}

resource "teleport_inference_model" "openai" {
  version = "v1"
  metadata = {
    name = "openai-model"
  }
  spec = {
    openai = {
      openai_model_id = "gpt5"
      base_url        = "http://localhost:8080/"
    }
  }
}
