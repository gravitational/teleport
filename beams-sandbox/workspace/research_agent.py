#!/usr/bin/env python3
"""
Research agent using Tavily search + OpenAI tool calling + LangSmith tracing.
"""

import json
import os
from openai import OpenAI
from tavily import TavilyClient
from langsmith import traceable

# LangSmith tracing - enabled automatically when these are set
os.environ.setdefault("LANGCHAIN_TRACING_V2", "true")
os.environ.setdefault("LANGCHAIN_PROJECT", "beams-research-agent")

openai_client = OpenAI()
tavily_client = TavilyClient()

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "search",
            "description": "Search the web for current information",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search query"}
                },
                "required": ["query"],
            },
        },
    }
]


def search(query: str) -> str:
    results = tavily_client.search(query, max_results=3)
    return "\n\n".join(
        f"{r['title']}\n{r['url']}\n{r['content']}" for r in results["results"]
    )


@traceable(name="research-agent")
def run_agent(question: str) -> str:
    messages = [{"role": "user", "content": question}]

    while True:
        response = openai_client.chat.completions.create(
            model="gpt-4o-mini",
            messages=messages,
            tools=TOOLS,
        )
        msg = response.choices[0].message
        messages.append(msg)

        # No tool calls - we have a final answer
        if not msg.tool_calls:
            return msg.content

        # Execute tool calls
        for tool_call in msg.tool_calls:
            args = json.loads(tool_call.function.arguments)
            print(f"  Searching: {args['query']}")
            result = search(args["query"])
            messages.append({
                "role": "tool",
                "tool_call_id": tool_call.id,
                "content": result,
            })


def run():
    print("Research Agent (type 'quit' to exit)\n")
    while True:
        question = input("Question: ").strip()
        if question.lower() in ("quit", "exit", "q"):
            break
        if not question:
            continue

        answer = run_agent(question)
        print(f"\nAnswer: {answer}\n")


if __name__ == "__main__":
    run()
