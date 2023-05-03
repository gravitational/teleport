import os
from typing import List
from langchain.agents import ConversationalChatAgent, Tool, AgentExecutor
from langchain.chat_models import ChatOpenAI

from langchain.chains import LLMChain, ConversationChain
from langchain.agents.conversational_chat.output_parser import ConvoOutputParser
from langchain.memory import ConversationBufferMemory

class Command:
    hosts: List[str]
    command: str

    def __init__(self):
        self.hosts = []
        self.command = ""

    def set_hosts(self, query: str):
        self.hosts = query.split(',')
        return self.get_status()

    def set_command(self, query: str):
        self.command = query
        return self.get_status()

    def get_status(self) -> str:
        result = ""
        if self.hosts:
            result += f"Hosts set to: {self.hosts}.  "
        else:
            result += "No Hosts set yet.  "
        if self.command:
            result += f"Command set to: {self.command}.  "
        else:
            result += "No Command set yet.  "
        return result


command = Command()

tools = [
    Tool(
        name="Set Hosts",
        func=command.set_hosts,
        description="Sets Hosts where the command should run. Takes a comma-separated list."
    ),
    Tool(
        name="Set Command",
        func=command.set_command,
        description="Sets Hosts where the command should run. Takes a comma-separated list."
    ),
]

PREFIX = """
You are Teleport, a tool that users can use to connect to Linux servers and run relevant commands, as well as have a conversation.
A Teleport cluster is a connectivity layer that allows access to a set of servers. Servers may also be referred to as nodes.
Nodes sometimes have labels such as "production" and "staging" assigned to them. Labels are used to group nodes together.
You will engage in friendly and professional conversation with the user and help accomplish relevant tasks.

Prepare the command as instructed by the user. Request more information if needed."""

SUFFIX = """TOOLS
------
Teleport can use the following tools to respond to the user requests:

{{tools}}

{format_instructions}

USER'S INPUT
--------------------
Here is the user's input (remember to respond with a markdown code snippet of a json blob with a single action, and NOTHING else):

{{{{input}}}}"""

api_key = os.getenv("OPENAI_API_KEY")

llm = ChatOpenAI(
    model_name="gpt-4",
    openai_api_key=api_key,
)

# This agent does not support memory

agent = ConversationalChatAgent.from_llm_and_tools(
    llm=llm,
    tools=tools,
    system_message=PREFIX,
    human_message=SUFFIX,
)

executor = AgentExecutor.from_agent_and_tools(agent=agent, tools=tools, verbose=True)
# executor.run(input="Show me the status of the git repository located in `~/work/teleport`.", chat_history=[])
executor.run(input="Show me the status of the git repository located in `~/work/teleport` on the `HugosMackBookPro.local` node.", chat_history=[])

print(f"Command status: {command.get_status()}")