// Main JavaScript for SMS Mailer Landing Page

$(document).ready(function() {
    // Mobile menu toggle
    $('#mobile-menu-button').click(function() {
        $('#mobile-menu').toggleClass('hidden mobile-menu-open');
    });

    // Close mobile menu when clicking outside
    $(document).click(function(event) {
        if (!$(event.target).closest('#mobile-menu-button, #mobile-menu').length) {
            $('#mobile-menu').addClass('hidden').removeClass('mobile-menu-open');
        }
    });

    // Smooth scrolling for anchor links
    $('a[href^="#"]').on('click', function(event) {
        if (this.hash !== '') {
            event.preventDefault();
            var hash = this.hash;
            $('html, body').animate({
                scrollTop: $(hash).offset().top - 80
            }, 800);
            
            // Close mobile menu if open
            $('#mobile-menu').addClass('hidden').removeClass('mobile-menu-open');
        }
    });

    // Demo video modal
    $('#demo button').click(function() {
        // In a real implementation, this would show a video modal
        alert('Video demo would play here!');
    });

    // Form submission handling
    $('#contact-form').submit(function(e) {
        e.preventDefault();
        
        // Get form values
        var name = $('#name').val();
        var email = $('#email').val();
        var message = $('#message').val();
        
        // Basic validation
        if (!name || !email || !message) {
            alert('Please fill in all fields');
            return;
        }
        
        // In a real implementation, this would send the form data to the server
        // For demo purposes, just show a success message
        $('#contact-form').html('<div class="p-4 bg-green-100 text-green-700 rounded-md"><i class="fas fa-check-circle mr-2"></i>Thank you for your message! We\'ll get back to you soon.</div>');
    });

    // Animate elements on scroll
    $(window).scroll(function() {
        $('.animate-on-scroll').each(function() {
            var position = $(this).offset().top;
            var scroll = $(window).scrollTop();
            var windowHeight = $(window).height();
            
            if (scroll + windowHeight > position + 100) {
                $(this).addClass('animate-fade-in');
            }
        });
    });

    // Initialize tooltips
    $('[data-toggle="tooltip"]').tooltip();

    // Add feature card hover effects
    $('.bg-white.p-6.rounded-lg').addClass('feature-card');

    // Add button hover effects
    $('.bg-primary, .bg-gray-100').addClass('btn-hover-effect');

    // Automatically update copyright year
    $('#current-year').text(new Date().getFullYear());
});