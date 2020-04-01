TAU = (2*Math.PI);
S_PER_DAY = 3600*24;

function I(start, end) {
  this.start = start;
  this.end = end;
}

// Paints the background of a clock in the svg group 'group', of radius
// 'radius', centered at the origin.
function appendClockBackground(group, radius) {
  // paint gray background circle
  var R0 = 0.95*radius,  // leave a tiny margin
      R1 = 0.66*R0;
  var arc = d3.arc()
    .innerRadius(R0)
    .outerRadius(R1);
  group.append("path")
    .datum({startAngle: 0, endAngle: TAU})
    .attr("d", arc)
    .classed("background", true);

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

  let line = d3.line()
    .defined(d => d.defined)
    .x(d => d.defined ? d.x : 0)
    .y(d => d.defined ? d.y : 0);
  let p = group.append("path")
    .datum(_tick_data)
    .attr("d", line)
    .classed("clocklines", true);
}

function appendTimeText(group, radius) {
  group.append("text")
    .attr("font-size", 0.2*radius + "pt")
    .attr("dy", 0.35 + "em")
    .classed("clock-text", true)
    .text(d => d.hours + "h " + d.minutes + "m");
}

function AppendClock(svg) {
  let rect = svg.node().getBoundingClientRect()
  let radius = Math.min(rect.width, rect.height)/2
  console.log("AppendClock | rect")
  console.log(rect)
  console.log("AppendClock | radius = " + radius)

  // TODO datum(d => d) passes the interval data from the svg element to the
  // group element. I think there might be a way to do this implicitly, but I
  // don't know it
  group = svg.append("g").datum(d => d)
  group.attr("transform", "translate(" + radius + "," + radius + ")")
  appendClockBackground(group, radius);
  appendTimeText(group, radius);

  // Paint work times
  let R0 = 0.95*radius,  // leave a tiny margin
      R1 = 0.66*R0;
  let arc = d3.arc()
    .innerRadius(R0)
    .outerRadius(R1)
    .startAngle(d => (d.start * TAU)/S_PER_DAY)
    .endAngle(d => (d.end * TAU)/S_PER_DAY);
  let paths = svg.selectAll("g").selectAll("path.worktimes").data(d => {
    let morning = (new Date(d.date)).getTime()/1000;
    let new_intervals = [];
     console.log(d);
    for (let i of d.intervals) {
      new_intervals.push({
        start: i.start - morning,
        end: i.end - morning,
      });
      console.log(new_intervals[new_intervals.length-1]);
    }
    console.log("AppendClock | new_intervals:");
    console.log(new_intervals);
    return new_intervals;
  });
  paths.enter().append("path")
    .attr("d", arc)
    .classed("worktimes", true);
}
