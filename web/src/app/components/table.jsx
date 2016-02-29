var React = require('react');

const GrvTableTextCell = ({rowIndex, data, columnKey, ...props}) => (
  <GrvTableCell {...props}>
    {data[rowIndex][columnKey]}
  </GrvTableCell>
);

var GrvTableCell = React.createClass({
  render(){
    var props = this.props;
    return props.isHeader ? <th key={props.key}>{props.children}</th> : <td key={props.key}>{props.children}</td>;
  }
});

var GrvTable = React.createClass({

  renderHeader(children){
    var cells = children.map((item, index)=>{
      return this.renderCell(item.props.header, {index, key: index, isHeader: true, ...item.props});
    })

    return <thead><tr>{cells}</tr></thead>
  },

  renderBody(children){
    var count = this.props.rowCount;
    var rows = [];
    for(var i = 0; i < count; i ++){
      var cells = children.map((item, index)=>{
        return this.renderCell(item.props.cell, {rowIndex: i, key: index, isHeader: false, ...item.props});
      })

      rows.push(<tr key={i}>{cells}</tr>);
    }

    return <tbody>{rows}</tbody>;
  },

  renderCell(cell, cellProps){
    var content = null;
    if (React.isValidElement(cell)) {
       content = React.cloneElement(cell, cellProps);
     } else if (typeof props.cell === 'function') {
       content = cell(cellProps);
     }

     return content;
  },

  render() {
    var children = [];
    React.Children.forEach(this.props.children, (child, index) => {
      if (child == null) {
        return;
      }

      if(child.type.displayName !== 'GrvTableColumn'){
        throw 'Should be GrvTableColumn';
      }

      children.push(child);
    });

    var tableClass = 'table ' + this.props.className;

    return (
      <table className={tableClass}>
        {this.renderHeader(children)}
        {this.renderBody(children)}
      </table>
    );
  }
})

var GrvTableColumn = React.createClass({
  render: function() {
    throw new Error('Component <GrvTableColumn /> should never render');
  }
})

export default GrvTable;
export {GrvTableColumn as Column, GrvTable as Table, GrvTableCell as Cell, GrvTableTextCell as TextCell};
