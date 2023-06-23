/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import {
	DetailedHTMLProps,
	AnchorHTMLAttributes,
	ButtonHTMLAttributes,
} from "react";

import styled from 'styled-components';

export type ButtonVariant = "primary" | "secondary" | "secondary-white";
export type ButtonShape = "sm" | "md" | "lg" | "outline";

export interface BaseProps {
	variant?: ButtonVariant | ButtonVariant[];
	shape?: ButtonShape | ButtonShape[];
}

type DefaultButtonProps = DetailedHTMLProps<
	ButtonHTMLAttributes<HTMLButtonElement>,
	HTMLButtonElement
>;

type DefaultAnchorProps = DetailedHTMLProps<
	AnchorHTMLAttributes<HTMLAnchorElement>,
	HTMLAnchorElement
>;

type ButtonProps = BaseProps & DefaultButtonProps & { as: "button" };
type AnchorProps = BaseProps & DefaultAnchorProps & { as: "link" };

export const Button = ({
	as,
	variant,
	shape,
	className,
	...restProps
}: ButtonProps | AnchorProps) => {
	const props = {
		...restProps,
		className: `variant-${variant} shape-${shape} ${className ? className : ''}`
	};

	if (as === "link") {
		return <Wrapper {...props}/>;
	}

	return <Wrapper {...props}/>;
};

Button.defaultProps = {
	variant: "primary",
	shape: "md",
	as: "button",
};

const Wrapper = styled(props => props.as === 'button' ? <button {...props}/> : <a {...props}/>)`
	appearance: none;
	display: inline-flex;
	justify-content: center;
	align-items: center;
	box-sizing: border-box;
	border-style: solid;
	border-radius: 4px;
	overflow: hidden;
	font-weight: 600;
	white-space: nowrap;
	text-decoration: none;
	cursor: pointer;
	transition: background 300ms, color 300ms;

	&:focus {
		outline: none;
	}

	&:active {
		transition-duration: 0s;
		opacity: 0.56;
	}

	,
	&&:disabled {
		background-color: #f0f2f4;
		border-color: #f0f2f4;
		color: #455a64;
		cursor: default;
		pointer-events: none;
	}

	&.variant-primary {
		background-color: #512fc9;
		border-color: #512fc9;
		color: #fff;

		&:focus,
		&:hover {
			background-color: #651fff;
			border-color: #651fff;
			color: #fff;
		}
	}

	&.variant-secondary {
		background-color: #fff;
		border-color: #512fc9;
		color: #512fc9;

		&:focus,
		&:hover {
			background-color: #fff;
			border-color: #651fff;
			color: #651fff;
		}
	}

	&.variant-secondary-white {
		background-color: transparent;
		border-color: #fff;
		color: #fff;

		&:hover,
		&:active,
		&:focus {
			background-color: rgba(255 255 255 / 12%);
		}
	}

	&.shape-sm {
		height: 24px;
		padding: 0 8px;
		font-size: 10px;
		text-transform: uppercase;
		border-width: 1px;
	}

	&.shape-md {
		height: 32px;
		padding: 0 24px;
		font-size: 14px;
		border-width: 1px;
	}

	&.shape-lg {
		height: 48px;
		padding: 0 48px;
		font-size: 16px;
		border-width: 1px;
	}

	&.shape-outline {
		height: 48px;
		padding: 0 48px;
		font-size: 16px;
		font-weight: 700;
		border-width: 1px;
	}

  &.cta {
    flex-shrink: 0;

    @media (max-width: 900px) {
      font-size: 16px;
      height: 56px !important;
      width: 100%;
      margin-top: 8px;
    }

    @media (min-width: 901px) {
      margin-right: 16px;
    }
  }
`;
