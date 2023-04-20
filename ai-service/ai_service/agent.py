import ai_service.data as data
from langchain.agents import Tool
from langchain.chains.conversation.memory import ConversationBufferMemory
from langchain.chat_models import ChatOpenAI
from langchain.agents import initialize_agent
from llama_index.indices.query.query_transform.base import DecomposeQueryTransform

from llama_index.langchain_helpers.agents import (
    LlamaToolkit,
    create_llama_chat_agent,
    IndexToolConfig,
    GraphToolConfig,
)

decompose_transform = DecomposeQueryTransform(
    llm_predictor, verbose=True
)
