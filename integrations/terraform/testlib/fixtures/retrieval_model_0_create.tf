resource "teleport_inference_model" "prereq" {
  version = "v1"
  metadata = {
    name = "bedrock-model"
  }
  spec = {
    openai = {
      openai_model_id = "gpt-4"
    }
  }
}

resource "teleport_retrieval_model" "test" {
  version = "v1"

  spec = {
    bedrock = {
      region           = "us-west-2"
      bedrock_model_id = "amazon.titan-embed-text-v2:0"
    }
    inference_model_name = "bedrock-model"
  }

  depends_on = [teleport_inference_model.prereq]
}
