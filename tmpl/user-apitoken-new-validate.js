function validate_fields() {
    if($("#name").val().trim() != ""){
        $("#add-btn").prop('disabled', false);
    } else {
        $("#add-btn").prop('disabled', true);
    }
    return;
}
$("#name").on('input', function() {
    validate_fields();
});
$("#add-btn").click(function() {
    validate_fields();
});
