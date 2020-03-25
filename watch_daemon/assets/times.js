function Day() {
  this.intervals = [];
  this.total_time = [];
}

// TODO make sure this works if the arg is the empty list
function toIntervals(times) {
  var start_day = new Date();
  start_day.setTime(times[0]);
  resetDay(start_day);
  
  var next_day = new Date();
  next_day.setTime(times[0]);
  next_day.setDate(next_day.getDate() + 1);
  resetDay(next_day);
  
  var start_i = 0;
  var intervals = [];
  var TIME_GAP = 25*60*1000;  // 25m in ms
  var MS_PER_1H = 60*60*1000;
  for (var i=1; i<=times.length; i++) {
    // Check if we need to close the current interval because there's a
    // gap >25m
    if (i == times.length || (times[i] - times[i-1]) > TIME_GAP) {
      // times[i] is the first time that's outside the interval starting
      // at timest[start_i]
      if ((times[i-1] - times[start_i]) > MS_PER_1H) {
        // The interval starting at times start_i has non-zero duration
        // (i.e. it consists of more times than just times[start_i])
        // Add it to the result.
        var start = times[start_i] - start_day.getTime(),
            end = times[i-1] - start_day.getTime();
        intervals.push(new I(start, end));
      }
      start_i = i;  // Start a new interval at i
    }
    
    // Check if we need to close the current interval because it crosses
    // the end of a day
    // if (times[i].getTime > )
  }
  return intervals;
}
