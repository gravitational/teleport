from llama_index import GPTSimpleVectorIndex, SimpleDirectoryReader
from llama_index import (
    GPTSimpleVectorIndex,
    ServiceContext,
)
from llama_index import LLMPredictor, ServiceContext
from langchain import OpenAI
from pathlib import Path

# common settings for text chunking etc
llm = OpenAI(model_name="text-davinci-003", temperature=0, max_tokens=512)
llm_predictor = LLMPredictor(llm=llm)
service_context = ServiceContext.from_defaults(
    llm_predictor=llm_predictor, chunk_size_limit=512
)

if __name__ == "__main__":
    # load Teleport docs for additional context
    print("loading documents...")
    docs_documents = SimpleDirectoryReader("../docs/pages", recursive=True).load_data()
    print("building index...")
    docs_index = GPTSimpleVectorIndex.from_documents(
        docs_documents, service_context=service_context
    )

    print("writing index to disk...")
    Path("trained").mkdir(parents=True, exist_ok=True)
    docs_index.save_to_disk("trained/docs_index.json")
    print("training complete!")
