var hour = -1;
var time = new Date().getTime();
var exec_basis = -1;
var weekday = -1;
var name = "-";
var url = "/{basis}/{name}";
var chosen_tab = 1;
var event_id = null;
var schedule = false;
var periodic_sel = -1;

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

$("#schedule").click(function(){
    schedule = $("#schedule").is(":checked");
    if($("#schedule").is(":checked")){
        $("#schedule-div").css("display","block");
    } else {
        $("#schedule-div").css("display","none");
        url = "/{basis}/{name}";
        exec_basis = 4;
    }
    $("#is_scheduled").val(schedule);
});

$("#datetimepicker1").on("dp.change", function(old_date) {
    time = Math.round(old_date.timeStamp / 1000);
    $("#time").val(time);   
});

$("#datetimepicker2").on("dp.change", function(old_date) {
    time = Math.round(old_date.timeStamp / 1000);  
    $("#time").val(time);   
});

$("#datetimepicker3").on("dp.change", function(old_date) {
    time = Math.round(old_date.timeStamp / 1000);
    $("#time").val(time);   
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
    event_id = $("#event-id").val();
    url = "/{basis}/{name}/{event}";
});

$("#weekday-sel").change(function() {
    weekday = $("#weekday-sel").val();
});

$("#name-tab1").change(function() {
    $("#name-tab2").val($("#name-tab1").val());
    $("#name-tab3").val($("#name-tab1").val());
    $("#task-name").val($("#name-tab2").val());
});

$("#name-tab2").change(function() {
    $("#name-tab1").val($("#name-tab2").val());
    $("#name-tab3").val($("#name-tab2").val());
    $("#task-name").val($("#name-tab2").val());
});

$("#name-tab3").change(function() {
    $("#name-tab1").val($("#name-tab3").val());
    $("#name-tab2").val($("#name-tab3").val());
    $("#task-name").val($("#name-tab2").val());
});

$(".full_url").click(function() {
    var base_url = $(this).attr('href');
    if(schedule){
        if(chosen_tab == 1){
            if(periodic_sel == -1){
                alert("Please select the type of periodicity.");
                return false;
            }
        } else if(chosen_tab == 3){
            exec_basis = 3;
        } else if(chosen_tab == 2){
            exec_basis = 5;
            if(event_id == null){
                alert("Please select an event.");
                return false;
            }
        }
    } else {
        exec_basis = 4;
    }
    $("#new-task-form").attr("action", base_url);    
    $("#execution_type").val(exec_basis+"");
});