import asyncio
import json
import logging

import grpc
from dotenv import load_dotenv
from langchain.chat_models import ChatOpenAI
from langchain.schema import AIMessage, HumanMessage, SystemMessage

import ai_service.gen.teleport.assistant.v1.assistant_pb2_grpc as assistant_grpc
import ai_service.model as model
from ai_service.gen.teleport.assistant.v1.assistant_pb2 import (
    CompleteRequest,
    CompletionResponse,
)

DEFAULT_HELLO_MESSAGE = "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via ChatGPT."


async def assistant_query(
    chat_llm: ChatOpenAI,
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
    result = await chat_llm.agenerate([messages])
    completion = result.generations[0][0].message.content

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
    def __init__(self) -> None:
        self.chat_llm = ChatOpenAI(model_name="gpt-4", temperature=0.5)

    async def Complete(
        self, request: CompleteRequest, context: grpc.aio.ServicerContext
    ):
        return await assistant_query(self.chat_llm, request)


async def serve():
    logging.info("Starting Assist service")

    server = grpc.aio.server()
    assistant_grpc.add_AssistantServiceServicer_to_server(AssistantServicer(), server)
    listen_addr = "127.0.0.1:50051"
    server.add_insecure_port(listen_addr)

    logging.info("Starting server on %s", listen_addr)
    await server.start()
    await server.wait_for_termination()


if __name__ == "__main__":
    load_dotenv(verbose=True)

    logging.basicConfig(level=logging.DEBUG)
    asyncio.run(serve())
