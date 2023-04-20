from ai_service.data import graph, node_index, docs_index
from ai_service.train import llm_predictor
from langchain.chains.conversation.memory import ConversationBufferMemory
from langchain.chat_models import ChatOpenAI
from llama_index.indices.query.query_transform.base import DecomposeQueryTransform
from langchain.agents import AgentExecutor, initialize_agent
from langchain.agents.agent_types import AgentType

from llama_index.langchain_helpers.agents import (
    LlamaToolkit,
    IndexToolConfig,
    GraphToolConfig,
)

# configuration for node tool
node_config = IndexToolConfig(
    index=node_index,
    name=f"Vector Index Nodes",
    description=f"useful for when you want to answer queries about Cluster Nodes",
    index_query_kwargs={"similarity_top_k": 5},
    tool_kwargs={"return_direct": False},
)

# configuration for docs tool
docs_config = IndexToolConfig(
    index=docs_index,
    name=f"Vector Index Docs",
    description=f"useful for when you want to answer queries about Teleport",
    index_query_kwargs={"similarity_top_k": 5},
    tool_kwargs={"return_direct": False},
)

# define a decompose transform for the graph
decompose_transform = DecomposeQueryTransform(llm_predictor, verbose=True)

# define query configs for graph
graph_query_configs = [
    {
        "index_struct_type": "simple_dict",
        "query_mode": "default",
        "query_kwargs": {
            "similarity_top_k": 3,
            # "include_summary": True
        },
        "query_transform": decompose_transform,
    },
    {
        "index_struct_type": "list",
        "query_mode": "default",
        "query_kwargs": {"response_mode": "tree_summarize", "verbose": True},
    },
]

# graph tool config
graph_config = GraphToolConfig(
    graph=graph,
    name=f"Graph Index",
    description="useful for when you want to answer questions about Teleport with nodes",
    query_configs=graph_query_configs,
    tool_kwargs={"return_direct": False},
)

# a toolkit groups together all the different indices and graphs, providing them as tools to an agent
toolkit = LlamaToolkit(
    index_configs=[node_config, docs_config], graph_configs=[graph_config]
)


# agent factory with a given LLM
def create_agent(
    chat_llm: ChatOpenAI, memory: ConversationBufferMemory
) -> AgentExecutor:
    llama_tools = toolkit.get_tools()
    return initialize_agent(
        llama_tools,
        chat_llm,
        agent=AgentType.CHAT_CONVERSATIONAL_REACT_DESCRIPTION,
        callback_manager=None,
        agent_path=None,
        agent_kwargs=None,
        memory=memory,
        verbose=True,
    )
