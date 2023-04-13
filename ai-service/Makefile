
grpc:
	mkdir -p ai_service/gen
	python -m grpc_tools.protoc -I../api/proto --python_out=ai_service/gen/ --pyi_out=ai_service/gen/ --grpc_python_out=ai_service/gen/ ../api/proto/teleport/assistant/v1/assistant.proto
