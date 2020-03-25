TAU = (2*Math.PI);
MS_PER_DAY = 24*60*60*1000;

function I(start, end) {
  this.start = start;
  this.end = end;
}

// Paints the background of a clock in the svg group 'group', of radius
// 'radius', centered at the origin.
function appendClockBackground(group, radius) {
  var R0 = 0.95*radius,  // leave a tiny margin
      R1 = 0.66*R0;
  var arc = d3.arc()
    .innerRadius(R0)
    .outerRadius(R1);
    
  // paint gray background circle
  group.append("path")
    .datum({startAngle: 0, endAngle: TAU})
    .attr("d", arc)
    .attr("class", "background");
    
  // Paint tick lines for each hour around the circle (each 1/24 of
  // the way around).
  //
  // Impl notes:
  // - I need to append three points for each hour (24): tick start point (inner
  //   radius), tick end point (inner radius plus epsilon), and undefined (to
  //   break the line, and make the ticks distinct--how d3.line() works)
  // - Every 6th tick is long, every 3rd tick is medium (if not long), and all
  //   others are short.
  // - We want to start painting the clock at the top (12 o'clock, or (0, -1)
  //   because y-coords are reversed in svg) and go clockwise, so swap sin & cos
  //  to reflect about the diagonal, and invert sin to reverse y-coordinates.
  let _tick_data = [];
  for (let i=0; i<24; i++) {
    let _tick_inner = R0,
        _tick_outer = R0 + ((i%6===0?0.5:i%3===0?0.3:0.15)*(R1-R0));
    console.log("_tick_inner: " + _tick_inner + " _tick_outer: " + _tick_outer);
    // Add three points to the path: beginning of tick, end of tick, undefined
    [ _tick_inner, _tick_outer ].forEach(R => {
      _tick_data.push({
        x: Math.sin((i*TAU)/24) * R,
        y: -Math.cos((i*TAU)/24) * R,
        defined: true
      });
    });
    _tick_data.push({defined: false});  // end each segment
  }
  console.log(_tick_data)
  
  let line = d3.line()
    .defined(d => d.defined)
    .x(d => d.defined?d.x:0)
    .y(d => d.defined?d.y:0);
  group.append("path")
    .datum(_tick_data)
    .attr("d", line)
    .attr("class", "clocklines");
}

function appendTimeText(group, intervals, radius) {
  let total_time=0;
  for (let i=0; i<intervals.length; i++) {
    total_time += intervals[i].end - intervals[i].start;
  }
  let minutes = Math.floor(total_time/(60*1000)),
      hours = Math.floor(minutes/60);
  minutes %= 60;
  group.append("text")
    .attr("font-size", 0.2*radius + "pt")
    .attr("dy", 0.35 + "em")
    .attr("class", "clock-text")
    .text(hours + "h " + minutes + "m");
}

function AppendClock(group, intervals, radius) {
  appendClockBackground(group, radius);
  appendTimeText(group, intervals, radius);
  let R0 = 0.95*radius,  // leave a tiny margin
      R1 = 0.66*R0;
  let arc = d3.arc()
    .innerRadius(R0)
    .outerRadius(R1);
  // Paint work times
  arc.startAngle(d => (d.start * TAU)/MS_PER_DAY);
  arc.endAngle(d => (d.end * TAU)/MS_PER_DAY);
  group.selectAll("path.worktimes")
    .data(intervals)
    .enter()
    .append("path")
      .attr("d", arc)
      .attr("class", "worktimes");
}

// function addLine(group, radius, time, color) {
//   MS_PER_6H = 6*60*60*1000;  // == MS_PER_DAY/4
//   var x0 = 0.5*radius*Math.cos(((time - MS_PER_6H)*TAU)/MS_PER_DAY),
//       y0 = 0.5*radius*Math.sin(((time - MS_PER_6H)*TAU)/MS_PER_DAY),
//       x1 = radius*Math.cos(((time - MS_PER_6H)*TAU)/MS_PER_DAY),
//       y1 = radius*Math.sin(((time - MS_PER_6H)*TAU)/MS_PER_DAY);
//   if (color === undefined) color = "black";
//   group.append("path")
//     .datum([[x0, y0], [x1, y1]])
//     .attr("d", d3.line())
//     .attr("stroke", color);
// }

// compute the radius of the incircle of a rectangle with width 'width'
// and height 'height'
function inradius(width, height) {
  return Math.min(width, height)/2;
}

// groups = d3.selectAll("svg.timer").data(days).update().append("group")
// AppendClock(groups, ???, ???)
