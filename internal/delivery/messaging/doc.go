// Package messaging adalah delivery layer untuk pesan Kafka.
//
// Perannya sama dengan delivery HTTP: menerima input dari luar (pesan), lalu
// meneruskannya ke handler. Package ini juga mengurus hal teknis konsumsi
// pesan seperti consumer group dan commit offset.
package messaging
