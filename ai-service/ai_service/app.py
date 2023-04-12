import json
from flask import Flask, request
from langchain.llms import OpenAI
from langchain.chat_models import ChatOpenAI
from langchain import PromptTemplate, LLMChain
from langchain.prompts.chat import (
    ChatPromptTemplate,
    SystemMessagePromptTemplate,
    AIMessagePromptTemplate,
    HumanMessagePromptTemplate,
)
from langchain.schema import AIMessage, HumanMessage, SystemMessage
import ai_service.model as model

app = Flask(__name__)


@app.route("/")
def root():
    return "Hello, World!"


llm = OpenAI(model_name="gpt-4", temperature=0.1)
chat_llm = ChatOpenAI(temperature=0.1)


@app.route("/assistant_query", methods=["POST"])
def assistant_query():
    messages = model.context(username=request.json["username"])
    for raw_message in request.json["messages"]:
        match raw_message["kind"]:
            case "human":
                messages.append(HumanMessage(content=raw_message["text"]))
            case "ai":
                messages.append(AIMessage(content=raw_message["text"]))
            case "system":
                messages.append(SystemMessage(content=raw_message["text"]))

    completion = chat_llm(messages)
    return completion.content
