import { Component, createElement, PropTypes } from 'react'
import hoistStatics from 'hoist-non-react-statics'

const reactorShape = PropTypes.shape({
  dispatch: PropTypes.func.isRequired,
  evaluate: PropTypes.func.isRequired,
  evaluateToJS: PropTypes.func.isRequired,
  observe: PropTypes.func.isRequired,
})

function getDisplayName(WrappedComponent) {
  return WrappedComponent.displayName || WrappedComponent.name || 'Component'
}

export default function connect(mapStateToProps) {
  return function wrapWithConnect(WrappedComponent) {
    class Connect extends Component {
      constructor(props, context) {
        super(props, context)
        this.reactor = props.reactor || context.reactor
        this.unsubscribeFns = []
        this.updatePropMap(props)
      }

      resubscribe(props) {
        this.unsubscribe()
        this.updatePropMap(props)
        this.updateState()
        this.subscribe()
      }

      componentWillMount() {
        this.updateState()
        this.subscribe(this.props)
      }
      
      componentWillUnmount() {
        this.unsubscribe()
      }

      updatePropMap(props) {
        this.propMap = (mapStateToProps) ? mapStateToProps(props) : {}
      }

      updateState() {
        let propMap = this.propMap
        let stateToSet = {}

        for (let key in propMap) {
          const getter = propMap[key]
          stateToSet[key] = this.reactor.evaluate(getter)
        }

        this.setState(stateToSet)
      }

      subscribe() {
        let propMap = this.propMap
        for (let key in propMap) {
          const getter = propMap[key]
          const unsubscribeFn = this.reactor.observe(getter, val => {
            this.setState({
              [key]: val,
            })
          })

          this.unsubscribeFns.push(unsubscribeFn)
        }
      }

      unsubscribe() {
        if (this.unsubscribeFns.length === 0) {
          return
        }

        while (this.unsubscribeFns.length > 0) {
          this.unsubscribeFns.shift()()
        }
      }

      render() {
        return createElement(WrappedComponent, {
          reactor: this.reactor,
          ...this.props,
          ...this.state,
        })
      }
    }

    Connect.displayName = `Connect(${getDisplayName(WrappedComponent)})`
    Connect.WrappedComponent = WrappedComponent
    Connect.contextTypes = {
      reactor: reactorShape,
    }
    Connect.propTypes = {
      reactor: reactorShape,
    }

    return hoistStatics(Connect, WrappedComponent)
  }
}
