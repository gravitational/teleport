function Player(rid, term) {
    this.rid = rid;
    this.term = term;
    this.started = true;
    this.chunk = 1;
}

Player.prototype.start = function() {
    this.started = true;
    this.iterChunks(this.chunk);
}

Player.prototype.stop = function() {
    this.started = false;
}

Player.prototype.iterChunks = function(chunk) {
    console.log("iterChunks: ", chunk);
    var self = this;
    $.ajax({
        url: "/api/records/" +self.rid +"/chunks?"+$.param([{name: "start", value: chunk}, {name: "end", value: chunk+1}]),
        type: "GET",
        dataType: 'json',
        success: function(data) {
            if(data.length == 0) {
                self.term.write("end of playback");
                return
            }
            self.writeChunk(data, 0, function(){
                self.iterChunks(chunk+data.length);
            });
        }.bind(this),
        error: function(xhr, status, err) {
            toastr.error("failed to connect to server, try again");
        }.bind(this)
    });
}

Player.prototype.writeChunk = function(chunks, i, fin) {
    var self = this;
    if(!self.started) {
        return;
    }
    var ms = chunks[i].delay/1000000;
    setTimeout(function() {
        self.term.write(atob(chunks[i].data));
        if(i + 1 < chunks.length) {
            self.writeChunk(chunks, i + 1, fin);
        } else {
            fin();
        }
    }, ms);
}
