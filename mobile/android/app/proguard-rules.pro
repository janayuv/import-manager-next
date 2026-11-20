# Flutter Proguard Rules
-keep class io.flutter.app.** { *; }
-keep class io.flutter.plugin.** { *; }
-keep class io.flutter.util.** { *; }
-keep class io.flutter.view.** { *; }
-keep class io.flutter.** { *; }
-keep class io.flutter.plugins.** { *; }

# Sqflite database engine
-keep class com.tekartik.sqflite.** { *; }

# Cryptography / Security
-keep class com.google.crypto.tink.** { *; }

# Keep native methods and classes across all dependencies
-keepclasseswithmembernames class * {
    native <methods>;
}

-dontwarn io.flutter.embedding.**
-dontwarn com.tekartik.sqflite.**
