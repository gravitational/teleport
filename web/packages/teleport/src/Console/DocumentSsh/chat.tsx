import React, { useState, useEffect, useRef } from 'react';
import styled, { keyframes } from 'styled-components';
import { getAccessToken, getHostName } from 'teleport/services/api';
import { ServerMessage } from 'teleport/Assist/types';

const fadeIn = keyframes`
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
`;

const ChatContainer = styled.div`
  color: white;
  font-family: monospace;
  font-size: 14px;
  display: flex;
  flex-direction: column;
  height: 60%;
  background-color: #6e40c9;
  padding: 20px;
  width: 60vh;
  position: absolute;
  right: 0;
  bottom: 0;
  z-index: 9999;
`;

const ChatMessages = styled.div`
  flex: 1;
  overflow-y: scroll;
`;

const MessageContainer = styled.div`
  display: flex;
  flex-direction: column;
  align-items: ${props => (props.isOwnMessage ? 'flex-end' : 'flex-start')};
  margin-bottom: 10px;
`;

const MessageBubble = styled.div`
  background-color: ${props => (props.isOwnMessage ? '#4CAF50' : '#f2f2f2')};
  color: ${props => (props.isOwnMessage ? 'white' : 'black')};
  padding: 10px;
  border-radius: 10px;
  animation: ${fadeIn} 0.3s ease-in-out;
`;

const ChatInput = styled.div`
  display: flex;
  align-items: center;
  margin-top: auto;
  padding: 10px;
`;

const ChatInputField = styled.input`
  flex: 1;
  margin-right: 10px;
`;

const ChatInputButton = styled.button`
  background-color: #4caf50;
  color: white;
  border: none;
  padding: 10px;
  border-radius: 10px;
  cursor: pointer;
`;

const Prompt = styled.span`
  color: green;
`;

export interface ChatProps {
  pasteFn: (text: string) => void;
  getSuggestionsFn: () => string;
}

function Chat(props: ChatProps) {
  const [messages, setMessages] = useState([]);
  const [inputValue, setInputValue] = useState('');
  const [socket, setSocket] = useState(null);
  const socketUrl =
    `wss://${getHostName()}/v1/webapi/sites/ubuntu/assistant?action=ssh-cmdgen&access_token=` +
    getAccessToken();
  const socketRef = useRef(null);

  useEffect(() => {
    socketRef.current = new WebSocket(socketUrl);
    setSocket(socketRef.current);

    return () => {
      socketRef.current.close();
    };
  }, [socketUrl]);

  useEffect(() => {
    if (socket) {
      socket.onmessage = event => {
        const message = event.data;

        const msg = JSON.parse(message) as ServerMessage;
        console.log('msg', msg);
        if(msg.type === 'CHAT_PARTIAL_MESSAGE_ASSISTANT') {
          setMessages([...messages, msg.payload]); // TODO: We should append to the last message, but this is good enough for testing
          return;
        }
        const payload = JSON.parse(msg.payload) as {
          action: string;
          input: string;
          log: string;
          reasoning: string;
        };
        console.log('payload', payload);
        const innerPayload = JSON.parse(payload.input);
        const command = innerPayload.command;

        props.pasteFn(command);
        setMessages([...messages, payload.reasoning]);
      };
    }
  }, [socket, messages]);

  function handleInputChange(event) {
    setInputValue(event.target.value);

    if (event.key === 'Enter') {
      // check if the "Enter" key was pressed
      handleSendMessage();
    }
  }

  function handleSendMessage() {
    if (inputValue.trim() !== '') {
      socket.send(inputValue);
      setMessages([...messages, inputValue]);
      setInputValue('');

      console.log('handleSendMessage', inputValue);
    }
  }

  function explainOutput() {
    const output = props.getSuggestionsFn();
    if (!output) {
      console.log('no output');
      return;
    }

    // base64 the output
    const encodedOutput = btoa(unescape(encodeURIComponent(output)));

    const socketUrl =
      `wss://${getHostName()}/v1/webapi/sites/ubuntu/assistant?action=ssh-explain&access_token=` +
      getAccessToken();
    const ws = new WebSocket(socketUrl);
    ws.onopen = () => {
      console.log('ws explain open');
      ws.send(encodedOutput);
    };

    ws.onmessage = event => {
      const message = event.data;

      console.log('ws explain message', message);
      const msg = JSON.parse(message) as ServerMessage;
      setMessages([...messages, 'Command summary:\n' + msg.payload]);
    };
  }

  return (
    <ChatContainer>
      <ChatMessages>
        {messages.map((message, index) => (
          <MessageContainer key={index} isOwnMessage={false}>
            <MessageBubble isOwnMessage={false}>{message}</MessageBubble>
          </MessageContainer>
        ))}
      </ChatMessages>
      <ChatInput>
        <Prompt>$</Prompt>
        <ChatInputField
          type="text"
          value={inputValue}
          onChange={handleInputChange}
          onKeyDown={handleInputChange}
        />
        <ChatInputButton onClick={handleSendMessage}>Send</ChatInputButton>
        <ChatInputButton onClick={explainOutput}>Explain</ChatInputButton>
      </ChatInput>
    </ChatContainer>
  );
}

export default Chat;

`{"type":"CHAT_MESSAGE_PROGRESS_UPDATE",
"created_time":"2023-08-05T03:03:43Z",
"payload":"{\"action\":\"Command Generation\",\"input\":\"{\\\"command\\\":\\\"df -h\\\"}\",\"log\":\"\",\"reasoning\":\"This command will display the amount of disk space used and remaining on your Linux system in a human-readable format.\"}"}`;
