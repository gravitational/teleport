import json
import logging
from concurrent import futures

import grpc
from langchain.chat_models import ChatOpenAI
from langchain.schema import AIMessage, HumanMessage, SystemMessage

from ai_service.gen.teleport.assistant.v1.assistant_pb2 import (
    CompleteRequest,
    CompletionResponse,
)
import ai_service.gen.teleport.assistant.v1.assistant_pb2_grpc as assistant_grpc
import ai_service.model as model

chat_llm = ChatOpenAI(model_name="gpt-4", temperature=0.5)

DEFAULT_HELLO_MESSAGE = "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via ChatGPT."


def assistant_query(
    request: CompleteRequest,
) -> CompletionResponse:
    logging.debug(request)
    if request.messages is None:
        return CompletionResponse(
            kind="chat",
            content=DEFAULT_HELLO_MESSAGE,
        )

    messages = model.context(username=request.username)
    for raw_message in request.messages:
        match raw_message.role:
            case "user":
                messages.append(HumanMessage(content=raw_message.content))
            case "assistant":
                messages.append(AIMessage(content=raw_message.content))
            case "system":
                messages.append(SystemMessage(content=raw_message.content))

    model.add_try_extract(messages)
    completion = chat_llm(messages).content

    try:
        data = json.loads(completion)
        return CompletionResponse(
            kind="command",
            command=data["command"],
            nodes=data["nodes"],
            labels=data["labels"],
        )
    except json.JSONDecodeError:
        return CompletionResponse(kind="chat", content=completion)


class AssistantServicer(assistant_grpc.AssistantServiceServicer):
    def Complete(self, request, context):
        return assistant_query(request)


def serve():
    logging.info("Assist service started")

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    assistant_grpc.add_AssistantServiceServicer_to_server(AssistantServicer(), server)
    server.add_insecure_port("127.0.0.1:50052")
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    serve()
