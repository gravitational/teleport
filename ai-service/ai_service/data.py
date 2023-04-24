from llama_index import GPTSimpleVectorIndex, SimpleDirectoryReader
from llama_index import (
    GPTSimpleVectorIndex,
    GPTListIndex,
)
from llama_index.indices.composability import ComposableGraph
from ai_service.train import service_context

# standin for supplying live node data from Teleport (Go rewrite would change this drastically anyhow)
node_documents = SimpleDirectoryReader("./knowledge/nodes").load_data()
node_index = GPTSimpleVectorIndex.from_documents(
    node_documents, service_context=service_context
)
node_summary = "Teleport Cluster Nodes"

# load Teleport docs for additional context
docs_index = GPTSimpleVectorIndex.load_from_disk("trained/docs_index.json")
docs_summary = "Teleport Documentation"

# compose a graph allowing cross-analysis of node data and Teleport docs
graph = ComposableGraph.from_indices(
    GPTListIndex,
    [node_index, docs_index],
    index_summaries=[node_summary, docs_summary],
    service_context=service_context,
)
