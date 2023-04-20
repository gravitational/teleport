from ai_service.agent import create_agent
from langchain.chat_models import ChatOpenAI
from langchain.memory import ConversationBufferMemory

chat_llm = ChatOpenAI(model_name="gpt-4", temperature=0.3)
memory = ConversationBufferMemory(memory_key="chat_history", return_messages=True)
agent = create_agent(chat_llm, memory)

while True:
    text_input = input("User: ")
    response = agent.run(input=text_input)
    print(f"Agent: {response}")
