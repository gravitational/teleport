import React from 'react';
import classnames from 'classnames';

const SizeEnum = {
  SM: 'sm',
  XS: 'xs',
  DEFAULT: 'default'
}

const styles = {
  size: {
    [SizeEnum.DEFAULT]: {
      minWidth: '60px'
    },
    [SizeEnum.SM]: {
      minWidth: '60px'
    }
  }
}

const getStyle = size => {
  return {
    ...styles.size[size]
  }
}

const hideContentStyle = {
  visibility: 'collapse',
  height: '0px'
}

const onClick = (e, props) => {
  e.preventDefault();
  let {isProcessing, isDisabled} = props;
  if (isProcessing || isDisabled) {
    return;
  }

  props.onClick();
};

const Button = props => {
  const {
    size,
    title,
    isProcessing = false,
    isBlock = false,
    className = '',
    children,
    isDisabled = false} = props;

  const containerClass = classnames('btn', className, {
    'disabled': isDisabled,
    'btn-block': isBlock,
    'btn-sm': size === SizeEnum.SM,
    'btn-xs': size === SizeEnum.XS,
  });

  const containerStyle = getStyle(size);
  const contentStyle = isProcessing ? hideContentStyle : {};
  return (
    <button
      style={containerStyle}
      title={title}
      className={containerClass}
      onClick={e => onClick(e, props)}>
      {isProcessing && <i className="fa fa-cog fa-spin fa-lg" />}
      <div style={contentStyle}>
        {children}
      </div>
    </button>
  );
}

export default Button;