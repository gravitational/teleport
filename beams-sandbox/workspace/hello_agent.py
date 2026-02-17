#!/usr/bin/env python3
"""
Simple hello agent using OpenAI SDK.
Responds with a greeting in whatever language the user speaks.
"""

from openai import OpenAI

client = OpenAI()

SYSTEM_PROMPT = """You are a friendly greeting agent.
When the user says anything, respond with a warm hello greeting in the same language they used.
Keep your response short - just a greeting."""


def run_agent():
    print("Hello Agent (type 'quit' to exit)\n")

    messages = [{"role": "system", "content": SYSTEM_PROMPT}]

    while True:
        user_input = input("You: ").strip()
        if user_input.lower() in ("quit", "exit", "q"):
            break
        if not user_input:
            continue

        messages.append({"role": "user", "content": user_input})

        response = client.chat.completions.create(
            model="gpt-4o-mini",
            messages=messages,
        )

        reply = response.choices[0].message.content
        messages.append({"role": "assistant", "content": reply})

        print(f"Agent: {reply}\n")


if __name__ == "__main__":
    run_agent()
