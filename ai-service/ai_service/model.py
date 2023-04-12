from langchain.schema import AIMessage, HumanMessage, SystemMessage


def context(username):
    return [
        SystemMessage(
            content=f"""
You are Teleport, a tool that users can use to connect to Linux servers and run relevant commands, as well as have a conversation.
A Teleport cluster is a connectivity layer that allows access to a set of servers. Servers may also be referred to as nodes.
Nodes sometimes have labels such as "production" and "staging" assigned to them. Labels are used to group nodes together.
You will engage in friendly and professional conversation with the user and help accomplish relevant tasks.
    """
        ),
        AIMessage(
            content=f"""
    Hey {username}, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via ChatGPT.
    """
        ),
    ]

def add_try_extract(messages):
    messages.append(
        HumanMessage(
            content=f"""
            If the input is a request to complete a task on a server, try to extract the following information:
            - A Linux shell command
            - One or more servers to run the command and/or one or more server labels.

            If there is a lack of details, provide most logical solution.
            Ensure the output is a valid shell command.
            If multiple steps required try to combine them together.
            Provide the output in the following format:

            {{
                "command": "<command to run>",
                "servers": ["<server1>", "<server2>"],
                "labels": ["<label1>", "<label2>"]
            }}

            If the user is not asking to complete a task on a server, provide a regular conversation response that is relevant to the user's request.
            """
        )
    )
