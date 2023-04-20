from llama_index import GPTSimpleVectorIndex, SimpleDirectoryReader
from llama_index import (
    GPTSimpleVectorIndex,
    ServiceContext,
)
from llama_index import LLMPredictor, ServiceContext
from langchain.chat_models import ChatOpenAI
from llama_index.node_parser import SimpleNodeParser
from pathlib import Path
from langchain.text_splitter import NLTKTextSplitter

# common settings for text chunking etc
llm = ChatOpenAI(model_name="text-davinci-003", temperature=0, max_tokens=512)
llm_predictor = LLMPredictor(llm=llm)
node_parser = SimpleNodeParser(text_splitter=NLTKTextSplitter())
service_context_natural_language = ServiceContext.from_defaults(
    llm_predictor=llm_predictor, node_parser=node_parser
)
service_context = ServiceContext.from_defaults(
    llm_predictor=llm_predictor, chunk_size_limit=512
)

if __name__ == "__main__":
    # load Teleport docs for additional context
    print("loading documents...")
    docs_documents = SimpleDirectoryReader("../docs/pages", recursive=False).load_data()
    print("building index...")
    docs_index = GPTSimpleVectorIndex.from_documents(
        docs_documents, service_context=service_context_natural_language
    )

    print("writing index to disk...")
    Path("trained").mkdir(parents=True, exist_ok=True)
    docs_index.save_to_disk("trained/docs_index.json")
    print("training complete!")
