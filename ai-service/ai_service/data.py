from llama_index import GPTSimpleVectorIndex, SimpleDirectoryReader
from llama_index import (
    GPTSimpleVectorIndex,
    ServiceContext,
    GPTListIndex,
)
from llama_index import LLMPredictor, ServiceContext
from langchain import OpenAI
from llama_index.indices.composability import ComposableGraph

# common settings for text chunking etc
llm=OpenAI(model_name="text-davinci-003", temperature=0, max_tokens=512)
llm_predictor = LLMPredictor(llm=llm)
service_context = ServiceContext.from_defaults(llm_predictor=llm_predictor)

# standin for supplying live node data from Teleport (Go rewrite would change this drastically anyhow)
node_documents = SimpleDirectoryReader("./knowledge/nodes").load_data()
print(len(node_documents))
node_index = GPTSimpleVectorIndex.from_documents(
    node_documents, service_context=service_context
)
node_summary = "Cluster Nodes"

# load Teleport docs for additional context
docs_documents = SimpleDirectoryReader('../docs/pages', recursive=True).load_data()
print(len(docs_documents))
docs_index = GPTSimpleVectorIndex.from_documents(
    docs_documents, service_context=service_context
)
docs_summary = "Teleport Documentation"

# compose a graph allowing cross-analysis of node data and Teleport docs
graph = ComposableGraph.from_indices(
    GPTListIndex,
    [node_index, docs_index],
    index_summaries=[node_summary, docs_summary],
    service_context=service_context,
)
