import json
import logging
import threading
from concurrent import futures
from queue import Queue
from typing import Generator

import grpc
from dotenv import load_dotenv
from langchain.callbacks.base import CallbackManager
from langchain.callbacks.streaming_stdout import StreamingStdOutCallbackHandler
from langchain.chat_models import ChatOpenAI
from langchain.schema import AIMessage, HumanMessage, SystemMessage

import ai_service.gen.teleport.assistant.v1.assistant_pb2_grpc as assistant_grpc
import ai_service.model as model
from ai_service.gen.teleport.assistant.v1.assistant_pb2 import (
    CompleteRequest,
    CompletionResponse,
)


class ChainStreamHandler(StreamingStdOutCallbackHandler):
    def __init__(self, gen: Queue):
        super().__init__()
        self.gen = gen

    def on_llm_new_token(self, token: str, **kwargs):
        self.gen.put(token)


DEFAULT_HELLO_MESSAGE = "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via ChatGPT."


class AssistantServicer(assistant_grpc.AssistantServiceServicer):
    def __init__(self) -> None:
        self.queue = Queue()

        self.chat_llm = ChatOpenAI(
            model_name="gpt-4",
            temperature=0.5,
            streaming=True,
            callback_manager=CallbackManager([ChainStreamHandler(self.queue)]),
            verbose=True,  # Some BS, but when verbose is false, then the handler is not being called!!!!!
        )

    def Complete(self, request, context):
        logging.debug("Complete called")

        yield from assistant_query(self, request)


def assistant_query(
    assistant: AssistantServicer,
    request: CompleteRequest,
) -> Generator[CompletionResponse, None, None]:
    logging.debug(request)
    if request.messages is None:
        yield CompletionResponse(
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

    # Start a new thread as this is what a random GH issues told me to do it.
    chat_thread = threading.Thread(target=assistant.chat_llm, args=(messages,))
    model.add_try_extract(messages)

    chat_thread.start()

    # HACK: assembly the whole payload and check if we received the command.
    full_payload = ""
    while True:
        elem = assistant.queue.get()
        yield CompletionResponse(
            kind="chat",
            content=elem,
        )
        full_payload += elem
        assistant.queue.task_done()
        # Super iffy condition to check if streaming is done.
        if not chat_thread.is_alive() and assistant.queue.empty():
            break

    chat_thread.join()

    try:
        data = json.loads(full_payload)
        yield CompletionResponse(
            kind="command",
            command=data["command"],
            nodes=data["nodes"],
            labels=data["labels"],
        )
    except json.JSONDecodeError:
        pass


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    assistant_grpc.add_AssistantServiceServicer_to_server(AssistantServicer(), server)
    server.add_insecure_port("127.0.0.1:50052")
    server.start()

    logging.info("Assist service started")
    server.wait_for_termination()


if __name__ == "__main__":
    load_dotenv()

    logging.basicConfig(level=logging.DEBUG)
    serve()
