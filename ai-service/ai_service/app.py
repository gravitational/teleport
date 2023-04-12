import json
from langchain.llms import OpenAI
from langchain.chat_models import ChatOpenAI
from langchain import PromptTemplate, LLMChain
from langchain.prompts.chat import (
    ChatPromptTemplate,
    SystemMessagePromptTemplate,
    AIMessagePromptTemplate,
    HumanMessagePromptTemplate,
)
from langchain.schema import (
    AIMessage,
    HumanMessage,
    SystemMessage
)

llm = OpenAI(model_name="gpt-3.5-turbo", temperature=0.3)
chat_llm = ChatOpenAI(temperature=0.3)

def get_messages():
    messages = []
    with open("messages.json", "r") as f:
        raw = json.load(f)
        for raw_message in raw["messages"]:
            match raw_message["kind"]:
                case "human":
                    messages.append(HumanMessage(content=raw_message["text"]))
                case "ai":
                    messages.append(AIMessage(content=raw_message["text"]))
                case "system":
                    messages.append(SystemMessage(content=raw_message["text"]))

    return messages

def run():
    messages = get_messages()
    completion = chat_llm(messages)
    print(completion.content)
