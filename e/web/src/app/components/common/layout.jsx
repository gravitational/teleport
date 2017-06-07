import React from 'react';

const styles = {
  flex: {
    display: 'flex'
  },

  justify: {
    start: {
      justifyContent: 'flex-start' 
    },

    end: {
      justifyContent: 'flex-end' 
    },
    
    between: {
      justifyContent: 'space-between'
    }
  },

  align: {
    center: {
      alignItems: 'center'
    },

    start: {
      alignItems: 'flex-start'
    },

    end: {
      alignItems: 'flex-end'
    },

    baseline: {
      alignItems: 'baseline'
    }
  },

  dir: {

    row: {
      flexDirection: 'row'
    },

    col: {
      flexDirection: 'column'
    }
  }
}

const getStyle = ({ dir='col', align='start', justify='start', style={} }) => {
  return {
    ...style,
    ...styles.flex,
    ...styles.dir[dir],
    ...styles.justify[justify],
    ...styles.align[align]
  }
}

const Flex = ({ className='', children, ...props }) => (
  <div className={className} style={getStyle(props)}>
    {children}
  </div>
)

export default {
  Flex
}
