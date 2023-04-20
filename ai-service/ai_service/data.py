from llama_index import GPTSimpleVectorIndex, SimpleDirectoryReader
from llama_index import download_loader, GPTSimpleVectorIndex, ServiceContext, GPTListIndex
from pathlib import Path
from llama_index import LLMPredictor, ServiceContext
from langchain import OpenAI
from llama_index.indices.composability import ComposableGraph

# used via LlamaHub to parse Teleport docs
UnstructuredReader = download_loader("UnstructuredReader", refresh_cache=True)
unstructured_loader = UnstructuredReader()

# common settings for text chunking etc
llm_predictor = LLMPredictor(llm=OpenAI(temperature=0, max_tokens=512))
service_context = ServiceContext.from_defaults(llm_predictor=llm_predictor)

# standin for supplying live node data from Teleport (Go rewrite would change this drastically anyhow)
node_documents = SimpleDirectoryReader('./knowledge/nodes').load_data()
node_index = GPTSimpleVectorIndex.from_documents(node_documents, service_context=service_context)
node_summary = "Cluster Nodes"

# load Teleport docs for additional context
docs_documents = unstructured_loader.load_data(file=Path('../docs/pages'), split_documents=False)
docs_index = GPTSimpleVectorIndex.from_documents(docs_documents, service_context=service_context)
docs_summary = "Teleport Documentation"

# compose a graph allowing cross-analysis of node data and Teleport docs
graph = ComposableGraph.from_indices(
    GPTListIndex,
    [node_index, docs_index], 
    index_summaries=[node_summary, docs_summary],
    service_context=service_context,
)
