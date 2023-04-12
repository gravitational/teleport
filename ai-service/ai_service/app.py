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


# llm = OpenAI(model_name="gpt-4", temperature=0)
chat_llm = ChatOpenAI(model_name="gpt-4", temperature=0.5)


@app.route("/assistant_query", methods=["POST"])
def assistant_query():
    print(request.json)
    if request.json["messages"] is None:
        return "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via ChatGPT."

    messages = model.context(username=request.json["username"])
    for raw_message in request.json["messages"]:
        match raw_message["role"]:
            case "user":
                messages.append(HumanMessage(content=raw_message["content"]))
            case "assistant":
                messages.append(AIMessage(content=raw_message["content"]))
            case "system":
                messages.append(SystemMessage(content=raw_message["content"]))

    model.add_try_extract(messages)
    completion = chat_llm(messages)
    return completion.content
