// Generate 1-5 random time intervals
function randomTimes() {
  var ct = Math.floor(Math.random() * 5) + 1;  // 1-5
  var max = 24;
  var nums = [];
  // Each interval is two numbers so need to gen. 2*ct nums.
  // This is a (ct*2)-element sample of [1,24], which is why "max" is
  // decremented, and nums is maintained as a sorted array.
  for (var i=0; i<(ct*2); i++) {
    var num = Math.floor(Math.random()*max) + 1;  // random()*max \in [0,max)
    max--;
    for (var j=0; j<=nums.length; j++) {
      if (j == nums.length || nums[j] > num) {
        nums.splice(j, 0, num);
        break;
      } else {
        num++;
      }
    }
  }
  
  var result = [];
  for (i=0; i<ct; i++) {
    result.push(new I(nums[2*i], nums[(2*i)+1]));
  }
  return result;
}

// Reset the date 'd' to the beginning of the same day
function resetDay(d) {
  d.setHours(0);
  d.setMinutes(0);
  d.setSeconds(0);
  d.setMilliseconds(0);
}

// Compute a list of save times generated by a poisson process with
// lambda = 1/l_inv
// TODO: pick a distribution that clusters better than exponential
function randomSaveTimes(end, duration) {
  // Compute inverse of CDF of exponential distribution = (1-e^(-lx))
  // y = 1-e^(-lx) => e^-lx = 1-y
  // => -lx = log(1-y) => x = -log(1-y)/l
  var l_inv = 10,
      start = new Date();
      start.setTime(end.getTime() - duration);
      times = [];
  for (var st = start.getTime();;) {
    var dt = l_inv*-Math.log(1-Math.random());  // Draw #mins from exp
    dt *= (60*1000);  // convert minutes to ms
    st += dt;
    if (st > end.getTime()) break;
    times.push(st);
  }
  return times;
}