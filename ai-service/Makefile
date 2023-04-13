
grpc:
	mkdir -p ai_service/gen
	python -m grpc_tools.protoc -I../api/proto --python_out=ai_service/gen/ --pyi_out=ai_service/gen/ --grpc_python_out=ai_service/gen/ ../api/proto/teleport/assistant/v1/assistant.proto
	# Fix imports as Python generator doesn't allow overrides https://github.com/protocolbuffers/protobuf/issues/7061
	# brew install gnu-sed if you're on Mac. sed -i as it doesn't there as expected. https://stackoverflow.com/questions/5694228/sed-in-place-flag-that-works-both-on-mac-bsd-and-linux
	sed -i "s|^from teleport.assistant.v1 import|from ai_service.gen.teleport.assistant.v1 import|" ai_service/gen/teleport/assistant/v1/assistant_pb2_grpc.py

fmt:
	black --exclude ai_service/gen ai_service/

run:
	python ai_service/app.py