from llama_index import GPTSimpleVectorIndex, SimpleDirectoryReader
from llama_index import (
    download_loader,
    GPTSimpleVectorIndex,
    ServiceContext,
    GPTListIndex,
)
from pathlib import Path
from llama_index import LLMPredictor, ServiceContext
from langchain import OpenAI
from llama_index.indices.composability import ComposableGraph
from llama_index.indices.query.query_transform.base import DecomposeQueryTransform
from llama_index.langchain_helpers.agents import (
    LlamaToolkit,
    create_llama_chat_agent,
    IndexToolConfig,
    GraphToolConfig,
)


# used via LlamaHub to parse Teleport docs
UnstructuredReader = download_loader("UnstructuredReader", refresh_cache=True)
unstructured_loader = UnstructuredReader()

# common settings for text chunking etc
llm_predictor = LLMPredictor(llm=OpenAI(temperature=0, max_tokens=512))
service_context = ServiceContext.from_defaults(llm_predictor=llm_predictor)

# standin for supplying live node data from Teleport (Go rewrite would change this drastically anyhow)
node_documents = SimpleDirectoryReader("./knowledge/nodes").load_data()
node_index = GPTSimpleVectorIndex.from_documents(
    node_documents, service_context=service_context
)
node_summary = "Cluster Nodes"
nodeconfig = IndexToolConfig(
    index=node_index, 
    name=f"Vector Index Nodes",
    description=f"useful for when you want to answer queries about Cluster Nodes",
    index_query_kwargs={"similarity_top_k": 3},
    tool_kwargs={"return_direct": True}
)

# load Teleport docs for additional context
docs_documents = unstructured_loader.load_data(
    file=Path("../docs/pages"), split_documents=False
)
docs_index = GPTSimpleVectorIndex.from_documents(
    docs_documents, service_context=service_context
)
docs_summary = "Teleport Documentation"
docs_config = IndexToolConfig(
    index=docs_index, 
    name=f"Vector Index Docs",
    description=f"useful for when you want to answer queries about Teleport",
    index_query_kwargs={"similarity_top_k": 3},
    tool_kwargs={"return_direct": True}
)

# compose a graph allowing cross-analysis of node data and Teleport docs
graph = ComposableGraph.from_indices(
    GPTListIndex,
    [node_index, docs_index],
    index_summaries=[node_summary, docs_summary],
    service_context=service_context,
)

# define a decompose transform for the graph
decompose_transform = DecomposeQueryTransform(
    llm_predictor, verbose=True
)

# define query configs for graph 
graph_query_configs = [
    {
        "index_struct_type": "simple_dict",
        "query_mode": "default",
        "query_kwargs": {
            "similarity_top_k": 1,
            # "include_summary": True
        },
        "query_transform": decompose_transform
    },
    {
        "index_struct_type": "list",
        "query_mode": "default",
        "query_kwargs": {
            "response_mode": "tree_summarize",
            "verbose": True
        }
    },
]

# graph tool config
graph_config = GraphToolConfig(
    graph=graph,
    name=f"Graph Index",
    description="useful for when you want to answer questions about Teleport with nodes",
    query_configs=graph_query_configs,
    tool_kwargs={"return_direct": True}
)
