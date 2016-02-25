var React = require('react');

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
        return this.renderCell(item.props.cell, {rowIndex: i, key: i, isHeader: false, ...item.props});
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

    return (
      <table className="table table-bordered">
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
export {GrvTableColumn as Column, GrvTable as Table, GrvTableCell as Cell};
