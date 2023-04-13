import json
import logging
from concurrent import futures

import grpc
from langchain.chat_models import ChatOpenAI
from langchain.schema import AIMessage, HumanMessage, SystemMessage

import ai_service.gen.teleport.assistant.v1.assistant_pb2 as assistant_pb2
import ai_service.gen.teleport.assistant.v1.assistant_pb2_grpc as assistant_pb2_grpc
import ai_service.model as model

chat_llm = ChatOpenAI(model_name="gpt-4", temperature=0.5)


def assistant_query(request: assistant_pb2.CompleteRequest):
    logging.debug(request)
    if request.messages is None:
        return {
            "kind": "chat",
            "content": "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via ChatGPT.",
        }

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
        return {
            "kind": "command",
            "command": data["command"],
            "nodes": data["nodes"],
            "labels": data["labels"],
        }
    except json.JSONDecodeError:
        return {"kind": "chat", "content": completion}


class AssistantServicer(assistant_pb2_grpc.AssistantServiceServicer):

    def Complete(self, request, context):
        resp = assistant_query(request)
        return assistant_pb2.CompletionResponse(**resp)

    def __init__(self) -> None:
        super().__init__()


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    assistant_pb2_grpc.add_AssistantServiceServicer_to_server(
        AssistantServicer(), server)
    server.add_insecure_port('[::]:50052')
    server.start()
    server.wait_for_termination()


if __name__ == '__main__':
    logging.basicConfig(level=logging.DEBUG)
    serve()
