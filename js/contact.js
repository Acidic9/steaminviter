jQuery(document).ready(function(){
	// JavaScript Starts Here //
	$("#contactForm").submit(function(){
		event.preventDefault();
		var name = $("#name").val();
		var email = $("#email").val();
		var message = $("#message").val();
		if (name.length <= 1) {
			alert("The name field is too short.");
			return false;
		} else if (!/^([a-zA-Z0-9_.+-])+\@(([a-zA-Z0-9-])+\.)+([a-zA-Z0-9]{2,4})+$/.test(email)) {
			alert("Your email seems incorrect.");
			return false;
		} else if (message.length < 20) {
			alert("Please write a message with 20 or more characters.")
			return false;
		}

		$("#contactForm input").each(function(){
			this.disabled = true;
		});
		$("#contactForm textarea").each(function(){
			this.disabled = true;
		});

		$.post("/sendMessage", {
			name: name,
			email: email,
			message: message
		}, function(result){
			if (result == "OK") {
				alert("Message sent successfully!");
			} else {
				$("#contactForm input").each(function(){
					this.disabled = false;
				});
				$("#contactForm textarea").each(function(){
					this.disabled = false;
				});
				alert("An error occured when trying to send your message.");
			}
		}).fail(function(){
			$("#contactForm input").each(function(){
				this.disabled = false;
			});
			$("#contactForm textarea").each(function(){
				this.disabled = false;
			});
			alert("An error occured when trying to send your message.")
		});
	});
});