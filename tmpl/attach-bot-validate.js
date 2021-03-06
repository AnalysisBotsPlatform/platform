var hour = -1;
var time = new Date().getTime();
var exec_basis = -1;
var weekday = -1;
var name = "-";
var url = "/{basis}/{name}";
var chosen_tab = 1;
var event_id = -1;
var schedule = false;
var periodic_sel = -1;

function pad(number, length){
    var str = "" + number
    while (str.length < length) {
        str = '0'+str
    }
    return str
}

$(function () {
    $('#datetimepicker1').datetimepicker({
        defaultDate: Date(),
        format: "HH:mm A"
    });
    $('#datetimepicker2').datetimepicker({
        defaultDate: Date(),
        format: "HH:mm A"
    });
    $('#datetimepicker3').datetimepicker({
        defaultDate: Date()
    });
});

$('#event-driven-tasks-tabs').click();
$('#periodic-tasks-tabs').click();
$(".dynamic-run").css("display","none");

$("#periodic-tasks-tabs").click(function(){
    chosen_tab = 1;
});

$("#event-driven-tasks-tabs").click(function(){
    chosen_tab = 2;
});

$("#one-time-tasks-tabs").click(function(){
    chosen_tab = 3;
});

$("#basis").change(function(){
    $(".dynamic-run").css("display","none");
    periodic_sel = $("#basis").val();
    if(periodic_sel == 0){
        $(".run-hourly").css("display","block");
        hour = $("#numberpicker-input").val()
        exec_basis = periodic_sel;
        url = "/{basis}/{name}/{hour}";
    } else if (periodic_sel == 1){
        $(".run-daily").css("display","block");
        exec_basis = periodic_sel;
        url = "/{basis}/{name}/{time}";
    } else if (periodic_sel == 2){
        $(".run-weekly").css("display","block");
        weekday = $("#weekday-sel").val();
        exec_basis = periodic_sel;
        url = "/{basis}/{name}/{weekday}/{time}";
    } else if(periodic_sel == -1) {
        exec_basis = 4;
        url = "/{basis}/{name}";
    }
});

$("#enable_scheduling").click(function(){
    schedule = true;
    $("#schedule-div").css("display","block");
    $("#disable_scheduling").css("display","block");
    $("#enable_scheduling").css("display","none");
});

$("#disable_scheduling").click(function(){
    schedule = false;
    exec_basis = 4;
    $("#schedule-div").css("display","none");
    $("#disable_scheduling").css("display","none");
    $("#enable_scheduling").css("display","block");
    $("#time").removeAttr('name');
    $("#type").removeAttr('name');
    $("#name").removeAttr('name');
    $("#cron").removeAttr('name');
});

$("#datetimepicker1").on("dp.change", function(old_date) {
    time = new Date(old_date.date).getTime();
});

$("#datetimepicker2").on("dp.change", function(old_date) {
    time = new Date(old_date.date).getTime();
});

$("#datetimepicker3").on("dp.change", function(old_date) {
    time = new Date(old_date.date).getTime();
});

$("#name").change(function() {
    name = $("#name").val();
});

$("#numberpicker-input").change(function() {
    hour = parseInt($("#numberpicker-input").val());
    if(hour <= 0){
        $("#numberpicker-input").val(1);
        hour = 0;
    } else if(hour >= 24){
        $("#numberpicker-input").val(23);
        hour = 23;
    } else if($("#numberpicker-input").val() == ""){
        $("#numberpicker-input").val(1);
        hour = 1;
    }
});

$("#event-id").change(function() {
    event_id = $("#event-id")[0].selectedIndex - 1;
    url = "/{basis}/{name}/{event}";
});

$("#weekday-sel").change(function() {
    weekday = $("#weekday-sel").val();
});

$("#name-tab1").change(function() {
    var n = $("#name-tab1").val();
    $("#name-tab2").val(n);
    $("#name-tab3").val(n);
    $('#name').attr('name', 'name');
    $('#name').val(n);
});

$("#name-tab2").change(function() {
    var n = $("#name-tab2").val();
    $("#name-tab1").val(n);
    $("#name-tab3").val(n);
    $('#name').attr('name', 'name');
    $('#name').val(n);
});

$("#name-tab3").change(function() {
    var n = $("#name-tab3").val();
    $("#name-tab1").val(n);
    $("#name-tab2").val(n);
    $('#name').attr('name', 'name');
    $('#name').val(n);
});

$(".full_url").click(function() {
    var base_url = $(this).attr('href');
    var cron = "";
    if(schedule){
        if(chosen_tab == 1){
            if(periodic_sel == -1){
                alert("Please select the type of periodicity.");
                return false;
            }
            if(periodic_sel == 0){
                cron = "0_*/"+hour+"_*_*_*";
            } else if(periodic_sel == 1){
                var date = new Date(time);
                var date_utc = new Date(date.getTime() + date.getTimezoneOffset() * 60000);
                cron = date_utc.getMinutes()+"_"+date_utc.getHours()+"_*_*_*";
            } else if(periodic_sel == 2){
                var date = new Date(time);
                var date_utc = new Date(date.getTime() + date.getTimezoneOffset() * 60000);
                cron = date_utc.getMinutes()+"_"+date_utc.getHours()+"_*_*_"+weekday;
            }
            $('#cron').attr('name', 'cron');
            $('#cron').val(cron);

            $('#time').removeAttr('name');
            $('#type').removeAttr('name');
        } else if(chosen_tab == 3){
            exec_basis = 3;
            $('#time').attr('name', 'time');
            
            var date = new Date(time);
            date.setSeconds(0);
            var date_utc = date.getTime() + date.getTimezoneOffset() * 60000;

            $("#time").val(date_utc);

            $('#cron').removeAttr('name');
            $('#type').removeAttr('name');
        } else if(chosen_tab == 2){
            exec_basis = 5;
            if(event_id == -1){
                alert("Please select an event.");
                return false;
            }
            $('#type').attr('name', 'type');
            $("#type").val(event_id);

            $('#time').removeAttr('name');
            $('#cron').removeAttr('name');
        }
    } else {
        exec_basis = 4;
        $('#name').removeAttr('name');
        $('#time').removeAttr('name');
        $('#type').removeAttr('name');
        $('#cron').removeAttr('name');
    }
    $("#new-task-form").attr("action", base_url);
    $("#execution_type").val(exec_basis+"");
});
